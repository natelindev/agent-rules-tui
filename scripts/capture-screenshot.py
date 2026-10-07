#!/usr/bin/env python3
"""Capture the actual TUI in a PTY, using disposable example projects only."""

import argparse
import codecs
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import shutil
import struct
import subprocess
import termios
import time

import pyte
from PIL import Image, ImageDraw, ImageFont

REPO = Path(__file__).resolve().parents[1]
COLS, ROWS = 124, 22


def font_path(override):
    candidates = [override] if override else [
        '/System/Library/Fonts/Menlo.ttc',
        '/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf',
        '/usr/share/fonts/truetype/liberation2/LiberationMono-Regular.ttf',
    ]
    for candidate in candidates:
        if candidate and Path(candidate).is_file():
            return candidate
    raise SystemExit('No monospaced font found; provide --font /path/to/font.ttf')


def create_fixture(root):
    groups = [
        ('atlas', ['AGENTS.md', 'CLAUDE.md', '.cursor/rules/frontend.mdc',
                   '.github/copilot-instructions.md']),
        ('payments-api', ['AGENTS.md', 'GEMINI.md', '.windsurf/rules/api.md']),
        ('global', ['AGENTS.md', 'CLAUDE.md']),
        ('frontend', ['AGENTS.md', '.cursorrules']),
        ('data-pipeline', ['AGENTS.md', 'QWEN.md']),
    ]
    for index, (name, files) in enumerate(groups):
        project = root / ('global' if name == 'global' else 'projects/' + name)
        project.mkdir(parents=True)
        if name != 'global':
            (project / '.git').mkdir()
        for relative in files:
            path = project / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text('# Example instructions\n\nUse focused changes and verify behavior.\n')
            timestamp = 1700000000 - index * 3600
            os.utime(path, (timestamp, timestamp))
    config = root / 'config.json'
    config.write_text(json.dumps({
        'editor': 'nvim',
        'roots': [str(root / 'projects')],
        'skip_dirs': ['.git', 'node_modules', 'dist'],
        'global_paths': [str(root / 'global')],
        'cache_path': str(root / 'cache.json'),
        'ignore_path_prompt': True,
    }))
    return config


def capture(binary, config):
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', ROWS, COLS, 0, 0))
    env = dict(os.environ, TERM='xterm-256color', COLORTERM='truecolor', CLICOLOR_FORCE='1')
    env.pop('NO_COLOR', None)
    process = None
    screen = pyte.Screen(COLS, ROWS)
    stream = pyte.Stream(screen)
    decoder = codecs.getincrementaldecoder('utf-8')(errors='replace')

    def pump(seconds):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            readable, _, _ = select.select([master], [], [], min(0.1, max(0, end - time.monotonic())))
            if readable:
                try:
                    data = os.read(master, 65536)
                except OSError:
                    break
                if not data:
                    break
                stream.feed(decoder.decode(data))

    try:
        process = subprocess.Popen([str(binary), '--config', str(config)],
                                   stdin=slave, stdout=slave, stderr=slave, env=env)
        os.close(slave)
        slave = None
        deadline = time.monotonic() + 15
        while not any('Found 13 files in 5 projects' in line for line in screen.display):
            pump(0.15)
            if process.poll() is not None or time.monotonic() > deadline:
                raise RuntimeError('TUI did not finish the example scan:\n' + '\n'.join(screen.display))
        # Expand atlas, select its Cursor rule, then expand the global group by click.
        os.write(master, b'\r')
        pump(0.3)
        # Rows: header (3), atlas (1), atlas files (4), payments (1), global (1).
        os.write(master, b'\x1b[<0;3;10M\x1b[<0;3;10m')
        pump(0.3)
        os.write(master, b'g')
        pump(0.15)
        os.write(master, b'j')
        pump(0.3)
        if not any('frontend.mdc' in line for line in screen.display):
            raise RuntimeError('Expanded example rules are missing from the terminal capture')
        return screen
    finally:
        if process is not None and process.poll() is None:
            os.write(master, b'q')
            try:
                process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                process.terminate()
                process.wait(timeout=3)
        os.close(master)
        if slave is not None:
            os.close(slave)


def render(screen, font, output):
    cell_width, cell_height = 12, 24
    padding, top, bottom = 30, 86, 42
    image = Image.new('RGB', (COLS * cell_width + padding * 2,
                             ROWS * cell_height + top + bottom), '#151821')
    draw = ImageDraw.Draw(image)
    face = ImageFont.truetype(font, 20)
    title_face = ImageFont.truetype(font, 16)
    draw.rectangle((0, 0, image.width, 54), fill='#1f2330')
    for x in (28, 50, 72):
        draw.ellipse((x, 23, x + 10, 33), fill='#647083')
    title = 'agent-rules / example workspace'
    draw.text(((image.width - draw.textlength(title, title_face)) / 2, 17),
              title, font=title_face, fill='#acb5c4')
    palette = {'black': '#161923', 'red': '#e06c75', 'green': '#98c379',
               'brown': '#e5c07b', 'blue': '#61afef', 'magenta': '#c678dd',
               'cyan': '#56b6c2', 'white': '#d9e0ec'}

    def color(value, default):
        if value == 'default':
            return default
        if value in palette:
            return palette[value]
        if len(value) == 6:
            return '#' + value
        return default

    for y in range(ROWS):
        for x in range(COLS):
            cell = screen.buffer[y][x]
            fg = color(cell.fg, '#d9e0ec')
            bg = color(cell.bg, '#151821')
            if cell.reverse:
                fg, bg = bg, fg
            left, upper = padding + x * cell_width, top + y * cell_height
            if bg != '#151821':
                draw.rectangle((left, upper, left + cell_width - 1, upper + cell_height - 1), fill=bg)
            if cell.data.strip():
                draw.text((left, upper), cell.data, font=face, fill=fg)
    output.parent.mkdir(parents=True, exist_ok=True)
    image.save(output, optimize=True)
    print(f'Saved {output} ({image.width} × {image.height})')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--font', help='Path to a monospaced TTF or TTC font')
    parser.add_argument('--output', type=Path, default=REPO / 'docs/assets/screenshot.png')
    args = parser.parse_args()
    font = font_path(args.font)
    binary = REPO / 'agent-rules'
    if not binary.is_file():
        parser.error('Build first: go build -o agent-rules ./cmd/agent-rules')
    # Stable, private, disposable path keeps example paths readable in the image.
    root = Path('/tmp/agent-rules-demo')
    try:
        root.mkdir(mode=0o700)
    except FileExistsError:
        parser.error(f'{root} already exists; refusing to modify it')
    try:
        config = create_fixture(root)
        render(capture(binary, config), font, args.output)
    finally:
        shutil.rmtree(root)


if __name__ == '__main__':
    main()
