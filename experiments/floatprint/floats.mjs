// Each host's default float-to-text, on the same doubles (win32org-2026-10-05 §4).
const xs = [0.1 + 0.2, 1.0, 100.0, 1e7, 1.0e21, 1e-7, 2e23, 5e-324,
            1.7976931348623157e308, 9007199254740993.0, -0.0, 1.0 / 3];
for (const x of xs) console.log(String(x));
