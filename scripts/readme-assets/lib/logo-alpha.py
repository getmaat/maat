"""Makes the logo's flat background transparent ("color to alpha"), so the
logo composites cleanly over gradients.

    python3 lib/logo-alpha.py <logo.webp> <out.png>
"""
import sys

import numpy as np
from PIL import Image

src, dst = sys.argv[1], sys.argv[2]
im = np.asarray(Image.open(src).convert('RGB')).astype(float)
bg = np.array([0, 6, 11], float)  # the logo's background color
ratio = np.where(im >= bg, (im - bg) / np.maximum(255 - bg, 1e-6), (bg - im) / np.maximum(bg, 1e-6))
alpha = np.clip(ratio.max(axis=2), 0, 1)
alpha = np.where(alpha < 0.035, 0, alpha)  # drop compression noise in the flat background
rgb = np.clip((im - bg) / np.maximum(alpha, 1e-6)[..., None] + bg, 0, 255)
Image.fromarray(np.dstack([rgb, alpha * 255]).astype(np.uint8), 'RGBA').save(dst)
