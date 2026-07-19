# Documentation du projet

Freedy est un projet Go de rendu 3D WebGPU visant, depuis une base de code
unique, une **fenêtre native** (GLFW) et un **canvas HTML (WASM)** — la seule
couture plateforme/rendu est `cmd/freedy`, qui passe un `*wgpu.SurfaceDescriptor`
à `renderer.New`.

Ce dépôt impose une méthode de documentation :

- Conventions (objectif, arborescence, découpe, artefacts, langue) :
  `.claude/rules/documentation.md`.
- Démarche : skill `doc` (`.claude/skills/doc/SKILL.md`) et ses templates.

`CLAUDE.md` et `README.md` existants font foi : la doc les reflète et les
développe, elle ne les réinvente pas ni ne les contredit.
