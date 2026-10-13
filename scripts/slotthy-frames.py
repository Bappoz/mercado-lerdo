"""Draws Slotthy's frames (32x32) from shapes: the source of SLOTTHY_FRAMES.

    python3 scripts/slotthy-frames.py <out dir>   # needs Pillow

writes sheet.png (a preview of every frame) and frames.ts (the rows to paste
into SLOTTHY_FRAMES, web/src/os/pixel.tsx). A new frame also needs its name
in SlotthyPose and a place in a clip (web/src/slotthyClips.ts).
"""
import json
import math
import sys

from PIL import Image

W = H = 32
N4 = ((1, 0), (-1, 0), (0, 1), (0, -1))
PAL = {
    "k": "#2b2670", "o": "#9c6b55", "d": "#6a4234", "l": "#bb8d74",
    "w": "#fbeede", "e": "#17133f", "p": "#f08fa0",
}


class Canvas:
    def __init__(self):
        self.g = [["."] * W for _ in range(H)]

    def paint(self, pts, col, outline=True):
        pts = {(x, y) for x, y in pts if 0 <= x < W and 0 <= y < H}
        if outline:
            for x, y in pts:
                for dx, dy in N4:
                    qx, qy = x + dx, y + dy
                    if (qx, qy) not in pts and 0 <= qx < W and 0 <= qy < H and self.g[qy][qx] != ".":
                        self.g[qy][qx] = "k"
        for x, y in pts:
            self.g[y][x] = col

    def px(self, col, *pts):
        for x, y in pts:
            if 0 <= x < W and 0 <= y < H:
                self.g[y][x] = col

    def finish(self):
        out = [row[:] for row in self.g]
        for y in range(H):
            for x in range(W):
                if self.g[y][x] != ".":
                    continue
                for dx, dy in N4:
                    qx, qy = x + dx, y + dy
                    if 0 <= qx < W and 0 <= qy < H and self.g[qy][qx] not in ".k":
                        out[y][x] = "k"
                        break
        return ["".join(r) for r in out]


def ellipse(cx, cy, rx, ry):
    return {
        (x, y)
        for y in range(H)
        for x in range(W)
        if ((x - cx) / rx) ** 2 + ((y - cy) / ry) ** 2 <= 1
    }


def seg(p, q, r):
    (x0, y0), (x1, y1) = p, q
    dx, dy = x1 - x0, y1 - y0
    L = dx * dx + dy * dy or 1
    out = set()
    for y in range(H):
        for x in range(W):
            t = max(0, min(1, ((x - x0) * dx + (y - y0) * dy) / L))
            if (x - x0 - t * dx) ** 2 + (y - y0 - t * dy) ** 2 <= r * r:
                out.add((x, y))
    return out


def limb(pts, r):
    out = set()
    for a, b in zip(pts, pts[1:]):
        out |= seg(a, b, r)
    return out


def claws(c, pts, spread=1.3, reach=2.3):
    """Three claws past the end of a limb, pointing the way it goes."""
    (x0, y0), (x1, y1) = pts[-2], pts[-1]
    d = math.hypot(x1 - x0, y1 - y0) or 1
    ux, uy = (x1 - x0) / d, (y1 - y0) / d
    tips = set()
    for s in (-spread, 0, spread):
        tips.add((math.floor(x1 + ux * reach - uy * s + 0.5), math.floor(y1 + uy * reach + ux * s + 0.5)))
    c.paint(tips, "w")


def mirror(pts):
    return [(31 - x, y) for x, y in pts]


ARMS = {
    # her right arm is on the screen's left (x small); the left one is mirrored
    "rest": [(9.5, 19.5), (6, 23), (5.5, 26.5)],
    "rest2": [(9.5, 19.5), (5.5, 23), (5, 26.5)],
    "point": [(9.5, 19.5), (6, 18.5), (2.8, 16.5)],
    "point2": [(9.5, 19.5), (6, 18), (2.8, 15.5)],
    "up": [(9.5, 19.5), (4.5, 17), (3.2, 12)],
    "up2": [(9.5, 19.5), (4, 16), (2.8, 10.5)],
    "wave": [(9.5, 19.5), (4.5, 16.5), (3.5, 10.5)],
    "wave2": [(9.5, 19.5), (4.5, 16), (2.2, 11.5)],
    "scratch": [(9.5, 19.5), (4, 15), (6.5, 6.5)],
    "scratch2": [(9.5, 19.5), (4, 14), (7.5, 5)],
    "mouth": [(9.5, 19.5), (7, 21), (11.5, 18)],
}
FRONT_ARMS = {"point", "point2", "up", "up2", "wave", "wave2", "scratch", "scratch2", "mouth"}

PATCH = {8: (12, 13), 9: (11, 13), 10: (9, 12), 11: (8, 11), 12: (7, 10), 13: (7, 9), 14: (7, 8)}


def front(bob=0, dy=0, eyes="open", mouth="smile", arm_r="rest", arm_l="rest", extra=None, look=0):
    c = Canvas()
    hy = bob + dy  # the head's shift

    def arm(name, left):
        pts = [(x, y + dy) for x, y in ARMS[name]]
        if left:
            pts = mirror(pts)
        c.paint(limb(pts, 1.7), "o")
        claws(c, pts)

    c.paint(ellipse(15.5, 23.5 + dy, 7.6, 6.6), "o")
    c.paint(ellipse(15.5, 24.5 + dy, 4.2, 4.6), "l", outline=False)
    for cx in (10.5, 20.5):
        c.paint(ellipse(cx, 29.2 + dy, 2.6, 1.4), "o")
    c.paint({(8, 30 + dy), (10, 30 + dy), (12, 30 + dy)}, "w", outline=False)
    c.paint({(19, 30 + dy), (21, 30 + dy), (23, 30 + dy)}, "w", outline=False)
    for name, left in ((arm_r, False), (arm_l, True)):
        if name not in FRONT_ARMS:
            arm(name, left)

    # head, tuft, face
    c.paint(ellipse(15.5, 10.5 + hy, 10.2, 8.2), "o")
    c.paint({(x, 2 + hy) for x in range(13, 19)} | {(14, 1 + hy), (15, 1 + hy), (17, 1 + hy)}, "o")
    face = ellipse(15.5, 11.6 + hy, 7.6, 5.7) - {(15, 6 + hy), (16, 6 + hy), (15, 5 + hy), (16, 5 + hy)}
    c.paint(face, "w", outline=False)
    for y, (a, b) in PATCH.items():
        for x in range(a, b + 1):
            c.px("d", (x, y + hy), (31 - x, y + hy))

    def both(col, *pts):
        for x, y in pts:
            c.px(col, (x + look, y + hy), (31 - x + look, y + hy))

    if eyes == "open":
        both("e", (10, 10), (11, 10), (10, 11), (11, 11))
        c.px("w", (11 + look, 10 + hy), (20 + look, 10 + hy))
    elif eyes == "half":
        both("e", (10, 11), (11, 11))
    elif eyes == "closed":
        both("e", (9, 11), (10, 11), (11, 11))
    elif eyes == "happy":
        both("e", (9, 12), (10, 11), (11, 11), (12, 12))
    elif eyes == "wide":
        both("w", (10, 10), (11, 10), (10, 11), (11, 11), (9, 10), (9, 11))
        both("e", (10, 11), (10, 10))
        both("k", (9, 7), (10, 7), (11, 8))  # brows, raised in the middle

    c.px("e", (14, 12 + hy), (15, 12 + hy), (16, 12 + hy), (17, 12 + hy), (15, 13 + hy), (16, 13 + hy))
    m = hy
    if mouth == "smile":
        c.px("k", (13, 14 + m), (18, 14 + m), (14, 15 + m), (15, 15 + m), (16, 15 + m), (17, 15 + m))
    elif mouth == "grin":
        c.px("k", (12, 14 + m), (19, 14 + m), (13, 15 + m), (18, 15 + m), (14, 16 + m), (15, 16 + m), (16, 16 + m), (17, 16 + m))
        c.px("p", (14, 15 + m), (15, 15 + m), (16, 15 + m), (17, 15 + m))
    elif mouth == "talk1":
        c.px("k", (14, 15 + m), (15, 15 + m), (16, 15 + m), (17, 15 + m), (15, 16 + m), (16, 16 + m))
    elif mouth == "talk2":
        c.px("k", (14, 14 + m), (15, 14 + m), (16, 14 + m), (17, 14 + m), (14, 15 + m), (17, 15 + m), (15, 16 + m), (16, 16 + m))
        c.px("p", (15, 15 + m), (16, 15 + m))
    elif mouth == "yawn1":
        c.px("k", (14, 14 + m), (15, 14 + m), (16, 14 + m), (17, 14 + m), (14, 15 + m), (17, 15 + m), (14, 16 + m), (15, 16 + m), (16, 16 + m), (17, 16 + m))
        c.px("p", (15, 15 + m), (16, 15 + m))
    elif mouth == "yawn2":
        c.px("k", (14, 14 + m), (15, 14 + m), (16, 14 + m), (17, 14 + m), (13, 15 + m), (18, 15 + m), (13, 16 + m), (18, 16 + m), (14, 17 + m), (15, 17 + m), (16, 17 + m), (17, 17 + m))
        c.px("e", (14, 15 + m), (15, 15 + m), (16, 15 + m), (17, 15 + m))
        c.px("p", (14, 16 + m), (15, 16 + m), (16, 16 + m), (17, 16 + m))
    elif mouth == "frown":
        c.px("k", (14, 15 + m), (15, 15 + m), (16, 15 + m), (17, 15 + m), (13, 16 + m), (18, 16 + m))
    elif mouth == "wobble":
        c.px("k", (13, 15 + m), (14, 16 + m), (15, 15 + m), (16, 16 + m), (17, 15 + m), (18, 16 + m))
    elif mouth == "flat":
        c.px("k", (14, 15 + m), (15, 15 + m), (16, 15 + m), (17, 15 + m))

    for name, left in ((arm_r, False), (arm_l, True)):
        if name in FRONT_ARMS:
            arm(name, left)
    for col, pts in extra or []:
        c.paint(set(pts), col)
    return c.finish()


def zed(x, y, big=False):
    if big:
        return [(x, y), (x + 1, y), (x + 2, y), (x + 3, y), (x + 2, y + 1), (x + 1, y + 2), (x, y + 3), (x + 1, y + 3), (x + 2, y + 3), (x + 3, y + 3)]
    return [(x, y), (x + 1, y), (x + 2, y), (x + 1, y + 1), (x, y + 2), (x + 1, y + 2), (x + 2, y + 2)]


def drop(x, y):
    return [(x, y), (x, y + 1), (x - 1, y + 1), (x + 1, y + 1), (x - 1, y + 2), (x, y + 2), (x + 1, y + 2)]


def side(phase, bob=0):
    """Walking, facing right: a slow diagonal gait in four frames."""
    c = Canvas()
    # (foot x offset, lifted) for near-front, near-hind, far-front, far-hind
    gait = [
        ((3, 0), (-2, 0), (-2, 0), (3, 0)),
        ((1, 0), (0, 2), (0, 2), (1, 0)),
        ((-2, 0), (3, 0), (3, 0), (-2, 0)),
        ((0, 2), (1, 0), (1, 0), (0, 2)),
    ][phase]
    by = 18.5 + bob
    hips = {"nf": (19, by + 2), "nh": (8, by + 2), "ff": (20.5, by + 1.5), "fh": (9.5, by + 1.5)}
    rest = {"nf": 20, "nh": 7, "ff": 22, "fh": 9}

    def leg(key, g, col):
        dx, up = g
        hx, hy = hips[key]
        fx, fy = rest[key] + dx, 27.5 - up
        c.paint(limb([(hx, hy), (fx, fy)], 1.8), col)
        bx, byy = math.floor(fx + 0.5), math.floor(fy + 0.5)
        c.paint({(bx + 2, byy + 1), (bx + 3, byy + 1), (bx + 3, byy)}, "w")

    leg("ff", gait[2], "d")
    leg("fh", gait[3], "d")
    c.paint(ellipse(13, by, 9.2, 5.6), "o")
    c.paint(ellipse(12.5, by + 2.6, 6, 2.6), "l", outline=False)
    c.paint(ellipse(4.2, by - 0.5, 1.3, 1.6), "o")  # a stub of a tail
    leg("nh", gait[1], "o")
    leg("nf", gait[0], "o")

    hy = bob
    head = ellipse(23, 11.5 + hy, 6.8, 6.3)
    c.paint(head, "o")
    c.paint({(x, 4 + hy) for x in range(20, 26)} | {(21, 3 + hy), (22, 3 + hy), (24, 3 + hy)}, "o")
    c.paint(ellipse(25.2, 12.6 + hy, 4.6, 4.3) & head, "w", outline=False)
    for x, y in ((24, 9), (25, 9), (23, 10), (24, 10), (25, 10), (22, 11), (23, 11), (24, 11), (25, 11), (21, 12), (22, 12), (23, 12), (21, 13), (22, 13), (28, 10), (29, 11), (28, 11)):
        c.px("d", (x, y + hy))
    c.px("e", (23, 11 + hy), (24, 11 + hy), (23, 10 + hy), (24, 10 + hy))
    c.px("w", (24, 10 + hy))
    c.px("e", (27, 12 + hy), (28, 12 + hy), (29, 12 + hy), (28, 13 + hy))
    c.px("k", (24, 14 + hy), (25, 15 + hy), (26, 15 + hy), (27, 15 + hy), (28, 14 + hy))
    return c.finish()


def frames():
    f = {}
    f["sit"] = front()
    f["breathe"] = front(bob=1, arm_r="rest2", arm_l="rest2")
    f["blinkHalf"] = front(eyes="half")
    f["blink"] = front(eyes="closed")
    f["lookLeft"] = front(look=-1)
    f["lookRight"] = front(look=1)
    f["talk1"] = front(mouth="talk1")
    f["talk2"] = front(mouth="talk2", bob=-1)
    f["happy1"] = front(eyes="happy", mouth="grin", arm_r="up", arm_l="up")
    f["happy2"] = front(eyes="happy", mouth="grin", arm_r="up2", arm_l="up2", bob=-1)
    f["happyTalk"] = front(eyes="happy", mouth="talk2", arm_r="up", arm_l="up")
    f["worried1"] = front(eyes="wide", mouth="frown", extra=[("w", drop(27, 3))])
    f["worried2"] = front(eyes="wide", mouth="wobble", bob=1, extra=[("w", drop(28, 6))])
    f["worriedTalk"] = front(eyes="wide", mouth="talk2", extra=[("w", drop(27, 3))])
    f["point1"] = front(arm_r="point", look=-1)
    f["point2"] = front(arm_r="point2", look=-1, mouth="talk1")
    f["wave1"] = front(eyes="happy", arm_l="wave")
    f["wave2"] = front(eyes="happy", arm_l="wave2", mouth="talk1")
    f["yawn1"] = front(eyes="half", mouth="yawn1")
    f["yawn2"] = front(eyes="closed", mouth="yawn2", bob=-1, arm_r="mouth")
    f["scratch1"] = front(eyes="half", mouth="flat", arm_l="scratch")
    f["scratch2"] = front(eyes="half", mouth="flat", arm_l="scratch2")
    f["doze"] = front(eyes="half", mouth="flat", bob=1)
    f["sleep1"] = front(eyes="closed", mouth="flat", bob=2, arm_r="rest2", arm_l="rest2", extra=[("w", zed(26, 6))])
    f["sleep2"] = front(eyes="closed", mouth="talk1", bob=1, extra=[("w", zed(27, 3))])
    f["sleep3"] = front(eyes="closed", mouth="flat", bob=2, arm_r="rest2", arm_l="rest2", extra=[("w", zed(24, 5)), ("w", zed(28, 1))])
    for i in range(4):
        f[f"walk{i + 1}"] = side(i, bob=i % 2)
    return f


def sheet(f, path, scale=8, cols=7):
    names = list(f)
    rows = -(-len(names) // cols)
    img = Image.new("RGB", (cols * (W + 2) * scale, rows * (H + 2) * scale), "#7fb0a0")
    px = img.load()
    for i, n in enumerate(names):
        ox, oy = (i % cols) * (W + 2) + 1, (i // cols) * (H + 2) + 1
        for y, row in enumerate(f[n]):
            for x, ch in enumerate(row):
                if ch == ".":
                    continue
                rgb = tuple(int(PAL[ch][j : j + 2], 16) for j in (1, 3, 5))
                for sy in range(scale):
                    for sx in range(scale):
                        px[(ox + x) * scale + sx, (oy + y) * scale + sy] = rgb
    img.save(path)


def ts(f):
    out = []
    for n, rows in f.items():
        out.append(f"  {n}: [")
        out += [f'    "{r}",' for r in rows]
        out.append("  ],")
    return "\n".join(out)


if __name__ == "__main__":
    out = sys.argv[1]
    f = frames()
    sheet(f, f"{out}/sheet.png")
    open(f"{out}/frames.ts", "w").write(ts(f) + "\n")
    json.dump(list(f), open(f"{out}/names.json", "w"))
    print(len(f), "frames")
