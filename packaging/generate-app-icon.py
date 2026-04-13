#!/usr/bin/env python3
from __future__ import annotations

import math
import shutil
import subprocess
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter


ROOT = Path(__file__).resolve().parents[1]
ASSETS_DIR = ROOT / "assets" / "icons"
MASTER_PNG = ASSETS_DIR / "token-manager-tools-icon-1024.png"
WINDOWS_ICO = ASSETS_DIR / "token-manager-tools.ico"
MACOS_ICNS = ASSETS_DIR / "token-manager-tools.icns"
LINUX_PNG = ASSETS_DIR / "token-manager-tools.png"
ICONSET_DIR = ASSETS_DIR / "token-manager-tools.iconset"

SIZE = 1024


def lerp(a: float, b: float, t: float) -> float:
    return a + (b - a) * t


def blend(c1: tuple[int, int, int], c2: tuple[int, int, int], t: float) -> tuple[int, int, int]:
    return tuple(int(lerp(c1[i], c2[i], t)) for i in range(3))


def draw_arrowhead(draw: ImageDraw.ImageDraw, center: tuple[float, float], angle_deg: float, color: tuple[int, int, int, int], scale: float) -> None:
    angle = math.radians(angle_deg)
    tip = center
    left = (
        center[0] - math.cos(angle - math.radians(24)) * scale,
        center[1] - math.sin(angle - math.radians(24)) * scale,
    )
    right = (
        center[0] - math.cos(angle + math.radians(24)) * scale,
        center[1] - math.sin(angle + math.radians(24)) * scale,
    )
    draw.polygon([tip, left, right], fill=color)


def draw_token(base: Image.Image, center: tuple[int, int], radius: int, fill: tuple[int, int, int], border: tuple[int, int, int, int]) -> None:
    shadow = Image.new("RGBA", base.size, (0, 0, 0, 0))
    shadow_draw = ImageDraw.Draw(shadow)
    x0, y0 = center[0] - radius, center[1] - radius
    x1, y1 = center[0] + radius, center[1] + radius
    shadow_draw.ellipse((x0 + 10, y0 + 18, x1 + 10, y1 + 18), fill=(6, 10, 18, 70))
    shadow = shadow.filter(ImageFilter.GaussianBlur(20))
    base.alpha_composite(shadow)

    token = Image.new("RGBA", base.size, (0, 0, 0, 0))
    token_draw = ImageDraw.Draw(token)
    token_draw.ellipse((x0, y0, x1, y1), fill=fill + (255,), outline=border, width=16)
    token_draw.ellipse((x0 + 34, y0 + 28, x1 - 34, y0 + radius - 8), fill=(255, 255, 255, 38))
    token_draw.arc((x0 + 34, y0 + 34, x1 - 34, y1 - 34), start=210, end=340, fill=(255, 255, 255, 56), width=12)
    base.alpha_composite(token)


def generate_icon() -> None:
    ASSETS_DIR.mkdir(parents=True, exist_ok=True)

    canvas = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))

    gradient = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    pixels = gradient.load()
    top_left = (22, 29, 38)
    bottom_right = (13, 78, 74)
    for y in range(SIZE):
        for x in range(SIZE):
            tx = x / (SIZE - 1)
            ty = y / (SIZE - 1)
            t = (tx * 0.62) + (ty * 0.38)
            color = blend(top_left, bottom_right, min(1.0, t))
            pixels[x, y] = color + (255,)

    mask = Image.new("L", (SIZE, SIZE), 0)
    ImageDraw.Draw(mask).rounded_rectangle((48, 48, SIZE - 48, SIZE - 48), radius=220, fill=255)
    canvas.paste(gradient, (0, 0), mask)

    glow = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    glow_draw = ImageDraw.Draw(glow)
    glow_draw.ellipse((120, 80, 700, 660), fill=(180, 255, 240, 54))
    glow_draw.ellipse((420, 420, 980, 980), fill=(90, 210, 200, 46))
    glow = glow.filter(ImageFilter.GaussianBlur(80))
    canvas.alpha_composite(glow)

    draw_token(canvas, (376, 382), 130, (120, 225, 196), (255, 255, 255, 56))
    draw_token(canvas, (630, 352), 112, (94, 203, 246), (255, 255, 255, 54))
    draw_token(canvas, (540, 628), 148, (41, 182, 149), (255, 255, 255, 48))

    ring = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    ring_draw = ImageDraw.Draw(ring)
    ring_draw.arc((170, 166, 862, 858), start=24, end=202, fill=(244, 247, 250, 255), width=56)
    ring_draw.arc((146, 146, 838, 838), start=220, end=372, fill=(164, 255, 228, 255), width=56)
    draw_arrowhead(ring_draw, (774, 308), -18, (244, 247, 250, 255), 86)
    draw_arrowhead(ring_draw, (298, 742), 164, (164, 255, 228, 255), 86)
    ring = ring.filter(ImageFilter.GaussianBlur(1))
    canvas.alpha_composite(ring)

    center = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    center_draw = ImageDraw.Draw(center)
    center_draw.ellipse((418, 418, 606, 606), fill=(12, 18, 26, 188), outline=(255, 255, 255, 56), width=10)
    center_draw.ellipse((452, 452, 572, 572), fill=(248, 252, 253, 230))
    center_draw.rounded_rectangle((470, 496, 554, 528), radius=16, fill=(22, 29, 38, 255))
    center_draw.rounded_rectangle((506, 458, 538, 566), radius=14, fill=(22, 29, 38, 255))
    center = center.filter(ImageFilter.GaussianBlur(0.4))
    canvas.alpha_composite(center)

    outline = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    outline_draw = ImageDraw.Draw(outline)
    outline_draw.rounded_rectangle((48, 48, SIZE - 48, SIZE - 48), radius=220, outline=(255, 255, 255, 48), width=3)
    canvas.alpha_composite(outline)

    canvas.save(MASTER_PNG)
    canvas.resize((512, 512), Image.Resampling.LANCZOS).save(LINUX_PNG)
    canvas.save(
        WINDOWS_ICO,
        sizes=[(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)],
    )

    if ICONSET_DIR.exists():
        shutil.rmtree(ICONSET_DIR)
    ICONSET_DIR.mkdir(parents=True)
    sizes = [16, 32, 128, 256, 512]
    for size in sizes:
        base_icon = canvas.resize((size, size), Image.Resampling.LANCZOS)
        base_icon.save(ICONSET_DIR / f"icon_{size}x{size}.png")
        if size != 512:
            base_icon_2x = canvas.resize((size * 2, size * 2), Image.Resampling.LANCZOS)
            base_icon_2x.save(ICONSET_DIR / f"icon_{size}x{size}@2x.png")
    canvas.save(ICONSET_DIR / "icon_512x512@2x.png")

    subprocess.run(
        ["iconutil", "-c", "icns", str(ICONSET_DIR), "-o", str(MACOS_ICNS)],
        check=True,
    )
    shutil.rmtree(ICONSET_DIR)


if __name__ == "__main__":
    generate_icon()
