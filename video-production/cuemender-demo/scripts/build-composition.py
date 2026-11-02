"""CueMender content in the user's exact ErrataScope demo visual system.

Run from any directory. Real captures remain separate, ignored assets.
The reference theme is copied from the user's original source, not inferred.
"""
from pathlib import Path
from html import escape
import json

ROOT = Path(__file__).resolve().parents[1]
CSS = (ROOT / 'assets/reference-theme.css').read_text(encoding='utf8')
CSS += '.footage img{position:absolute;width:100%;height:100%;object-fit:contain}.crop video,.crop img{width:100%;height:100%;object-fit:contain}.side h2 span,.hero .line,.subtitle .line{display:block}'
SCENES = []

def media(name, source, duration, offset=0, start=0):
    attrs = f'id="{name}" class="clip" data-start="{start}" data-duration="{duration}" data-track-index="2"'
    if source.endswith('.png'):
        return f'<img {attrs} data-hold src="assets/footage/{source}" alt="Actual CueMender capture">'
    return f'<video {attrs} data-media-start="{offset}" src="assets/footage/{source}" muted playsinline preload="auto"></video>'

def frame(title, badge, caption, body, simulated=False):
    color = ' style="color:#edbf65"' if simulated else ''
    return (body + f'<p class="top">{escape(title)}</p>'
            f'<span class="badge"{color}>{escape(badge)}</span>'
            f'<p class="caption" data-hold>{escape(caption)}</p>')

def footage(source, duration, offset=0, name='capture'):
    return '<div class="footage">' + media(name, source, duration, offset) + '</div>'

def focus(headline, body, detail, source, duration, width, height, name='focus'):
    k = min(824/width, 660/height)
    h = height*k
    return (f'<div class="side"><h2>{headline}</h2><p>{escape(body)}</p><p class="detail">{escape(detail)}</p></div>'
            f'<div class="crop" style="height:{h:.3f}px;top:{235+(660-h)/2:.3f}px">'
            + media(name, source, duration) + '</div>')

def add(name, duration, body, animation='', summary=''):
    start = sum(s['duration'] for s in SCENES)
    markup = (f'<template><div id="root" data-composition-id="{name}" data-duration="{duration}" data-width="1920" data-height="1080">'
              f'<style>{CSS}</style>{body}<script>const tl=gsap.timeline({{paused:true}});{animation}'
              f"window.__timelines['{name}']=tl;</script></div></template>\n")
    (ROOT/'compositions'/f'{name}.html').write_text(markup, encoding='utf8')
    SCENES.append(dict(id=name,start=start,duration=duration,summary=summary))

add('opening',8,
    '<p class="overline">CODEX × TWITCHCON · HACKATHON DEMONSTRATION</p>'
    '<h1 class="hero" id="intro-hero"><span>CueMender</span></h1>'
    '<p class="subtitle" id="intro-sub"><span class="line">Hear the report.</span><span class="line">See the evidence.</span></p>'
    '<div class="rule"></div><p class="name">Desan Vasu</p>'
    '<p class="minor">A local OBS diagnostic companion for Twitch streamers.</p>',
    "tl.fromTo('#intro-hero span',{y:24},{y:0,duration:.55,ease:'power2.out'},0);tl.fromTo('#intro-sub',{opacity:0,y:16},{opacity:1,y:0,duration:.5,ease:'power2.out'},.6);",
    'Exact reference opening geometry and motion; CueMender identity.')

add('overview',10,frame('From viewer reports to local OBS evidence','REAL · RECEIVE-ONLY',
    'Connected to real Twitch chat and local OBS. The channel is offline; no broadcast recovery is claimed.',
    footage('real-report.mp4',10,name='overview-capture')),summary='Actual monitoring UI in reference full-footage panel.')

add('chat',10,frame('A separate viewer reports an audio issue','REAL TWITCH CHAT',
    'The command shown was sent through Twitch by the separate viewer account. No typing is recreated.',
    focus('One report.<span>A fresh check.</span>','dvx6901 → desanv10','!issueaudio travels through the receive-only connection.',
          'chat-detail.mp4',10,700,150,'actual-chat')),summary='Actual Twitch message, reference split focus layout.')

add('report',20,frame('The report becomes a diagnostic incident','REAL · RECEIVE-ONLY',
    'CueMender checks the selected local OBS input and retains fresh evidence with the incident.',
    footage('real-arrival.mp4',20,name='actual-arrival')),summary='Real report arrival and incident creation, unchanged source time.')

add('evidence',14,frame('Read the actual OBS finding','REAL LOCAL OBS',
    'Controlled test: Mic/Aux was manually muted, then restored. CueMender does not automatically unmute it.',
    footage('finding-detail.mp4',14,name='actual-finding')),summary='Readable MIC_MUTED finding in full-width reference panel.')

add('overlay',10,frame('Keep the community informed','LOCAL OVERLAY PREVIEW',
    'The public overlay carries the same incident code. This is a local preview, not a broadcast capture.',
    focus('One incident.<span>Public status.</span>','CM-K8DBDW · actual finding','Private settings stay in the control surface.',
          'overlay-detail.mp4',10,550,180,'actual-overlay')),summary='Reference split focus; actual matching public overlay.')

add('boundary',10,
    '<p class="overline">CUEMENDER · RECEIVE-ONLY BOUNDARY</p>'
    '<h1 class="hero close-title"><span class="line">Real chat.</span><span class="line">Read-only evidence.</span></h1>'
    '<p class="subtitle close-sub" style="top:550px">No chat replies. No authenticated channel ownership.</p>'
    '<p class="name" style="top:710px">No real OBS repair through this connection.</p>'
    '<p class="minor" style="top:790px">Next: a separate, clearly simulated approval and repair rehearsal.</p>',
    summary='Reference title-card geometry, explicit capability boundary.')

add('proposal',10,frame('Review the exact proposed change','REHEARSAL / SIMULATED',
    'Simulated missing track: add only the selected microphone to track 2. Review the preserved assignments.',
    footage('proposal-detail.png',10,name='actual-proposal-still'),True),summary='Actual proposal still hold, full-width panel, amber simulation badge.')

add('approval',13,frame('Private approval precedes the simulated write','REHEARSAL / SIMULATED',
    'The rehearsal applies the selected-track addition and verifies its simulated read-back. No real OBS write.',
    footage('approval-detail.mp4',13,name='simulated-approval'),True),summary='Actual rehearsal approval/read-back recording.')

add('feedback',12,frame('Keep viewer feedback separate from local checks','REHEARSAL / SIMULATED',
    'A simulated viewer confirms the result; the operator then records an explicit closure outcome.',
    footage('feedback-detail.mp4',12,name='simulated-feedback'),True),summary='Actual rehearsal feedback and closure interaction.')

add('closed',5,frame('The outcome stays in the incident record','REHEARSAL / SIMULATED',
    'Rehearsal complete. Simulated feedback is not evidence that a real viewer heard repaired broadcast audio.',
    footage('rehearsal-closed.png',5,name='actual-closed-still'),True),summary='Authentic closed-incident screenshot hold.')

codex_media = ''.join([
    media('codex-request','codex-start.mp4',8,start=0),
    media('codex-build','codex-work.mp4',9,start=8),
    media('codex-debug','codex-debug-hold.mp4',8,start=17)])
add('codex',25,frame('Built with Codex: implementation and debugging','ACTUAL BUILD FOOTAGE',
    'Codex: approval engine and integration fixes. DeepSeek: other worker packets. These are genuine build excerpts.',
    '<div class="footage">'+codex_media+'</div>'),summary='Actual Codex excerpts, reference full-footage frame; no fictional test footage.')

add('limits',10,
    '<p class="overline">CUEMENDER · VERIFIED DEMO SCOPE</p>'
    '<h1 class="hero close-title"><span class="line">Real diagnostics.</span><span class="line">Simulated repair.</span></h1>'
    '<p class="subtitle close-sub" style="top:550px">Account phone verification blocked developer-app setup.</p>'
    '<p class="name" style="top:710px">Authenticated integration and real repair remain unverified.</p>'
    '<p class="minor" style="top:790px">The Windows preview includes receive-only, authenticated and isolated rehearsal modes.</p>',
    summary='Reference title-card layout with honest remaining limits.')

add('closing',8,
    '<p class="overline">CUEMENDER · WINDOWS PREVIEW v1.0.0</p>'
    '<h1 class="hero close-title" id="close-hero"><span class="line">Report the issue.</span><span class="line">Keep the evidence.</span></h1>'
    '<p class="subtitle close-sub" style="top:550px">Viewer report → OBS diagnosis → reviewable incident</p>'
    '<p class="name close-link" style="top:710px">github.com/desanv01/cuemender</p>'
    '<p class="minor" style="top:790px">Desan Vasu · Public source · MIT license · Windows release</p>',
    "tl.fromTo('#close-hero',{y:18},{y:0,duration:.5,ease:'power2.out'},0);",
    'Exact reference closing geometry and half-second title motion.')

duration = sum(s['duration'] for s in SCENES)
assert duration == 165
hosts = '\n'.join(f'<div id="{s["id"]}" class="clip" data-composition-id="{s["id"]}" data-composition-src="compositions/{s["id"]}.html" data-start="{s["start"]}" data-duration="{s["duration"]}" data-width="1920" data-height="1080" data-track-index="1" data-track-kind="graphics"></div>' for s in SCENES)
(ROOT/'index.html').write_text('<!doctype html><html lang="en"><head><meta charset="utf-8"><title>CueMender · Desan Vasu</title><script src="assets/vendor/gsap-3.14.2.min.js"></script><style>html,body{margin:0;width:1920px;height:1080px;overflow:hidden;background:#0d151c}#root{position:absolute;inset:0;width:100%;height:100%}.clip{position:absolute;inset:0}</style></head><body><div id="root" data-composition-id="main" data-duration="165" data-width="1920" data-height="1080" data-fps="30">'+hosts+"</div><script>window.__timelines['main']=gsap.timeline({paused:true});</script></body></html>\n",encoding='utf8')
(ROOT/'EDIT.json').write_text(json.dumps(dict(duration=duration,fps=30,reference='ErrataScope_Demo_Desan_Vasu.mp4',scenes=SCENES),indent=2)+'\n',encoding='utf8')
story = '# CueMender — ErrataScope reference revision\n\nSilent 165-second film. User selected the exact reference visual system; genuine content remains CueMender.\n\n'
for s in SCENES:
    story += f'## {s["start"]}–{s["start"]+s["duration"]}s: {s["id"]}\n\n{s["summary"]}\n\n'
(ROOT/'STORYBOARD.md').write_text(story.rstrip()+'\n',encoding='utf8')
print(f'Built {len(SCENES)} reference-matched scenes / {duration} seconds.')
