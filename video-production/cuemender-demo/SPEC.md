# CueMender film specification

GOAL: Follow one actual viewer report to an actual OBS finding; distinguish simulated repair from verified real behavior.
OUTPUT: HyperFrames 0.8.221, 1920x1080, 30 fps, H.264, silent.
TIME: 165 seconds = 4,950 frames. User-approved storyboard; no alternate concept exploration.
LOOK: User-selected ErrataScope reference, copied from its actual source: navy #0d151c ground, off-white #f8f5ee ink, cyan #65d2f4 accents and amber #edbf65 rehearsal badges. Local Inter 400/700. Hero 148px at (192,260), opening subtitle 44px at (198,460), author 38px at (198,670). Full footage panel 1728x864 at (96,114), header 32px at (96,40), badge 28px at right96/top43, caption 28px at (96,1000). Split focus: 700px text column at (192,285), 64px headline, right 824x660 media slot at (1000,235). Actual media is aspect-preserving and ungraded.
OBJECTS: real Twitch chat, actual CueMender incident and public overlay, actual rehearsal proposal/result, actual Codex build chat. No reconstructed application UI.
MOTION: Exact reference: intro hero y24 to0 in0.55s power2.out, already opaque; subtitle opacity0/y16 to1/y0 from0.6 to1.1s. Closing hero y18 to0 in0.5s power2.out. All other scene timelines are empty; authentic footage and direct cuts provide motion. No repeated header fade, zoom drift, transitions or animation added beyond the reference.
SHOTS: revised 14-scene storyboard in STORYBOARD.md; 165 seconds. Caption windows at least5seconds. User's exact reference overrides generic safe-area defaults: header/caption at96px horizontal and40/1000px vertical. The reference itself is152seconds; CueMender stays165seconds to preserve required real Twitch, rehearsal and Codex evidence.
BANNED: fabricated chat/tests, implied broadcast recovery, simulated-to-real repair continuity, animated fake cursor, tiny explanatory copy, music/voice, neon/glass/particles, decorative charts or KPI cards.
QA: all scene midpoints, exact real diagnostic transition, approval before/after, feedback/closure, Codex crop, final frame. Review encoded duration/decode, silent stream, and sample frames.

Intentional exceptions: actual UI text can be smaller than authored captions; original pixels are enlarged where useful. Authentic recordings and reading holds need not move continuously. Informative cyan/amber badges preserve the reference style and the real/rehearsal distinction. Reference analysis is documented in REFERENCE-ANALYSIS.md. No UI redesign is implied.
