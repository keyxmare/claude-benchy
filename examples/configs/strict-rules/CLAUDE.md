# app-under-test

Serveur HTTP Node.js minimal (`server.js`), sans dépendance externe.

## Règles

- Applique strictement `.claude/rules/style.md`.
- Tout nouvel endpoint est accompagné d'un test dans `server.test.js`
  (module `node:test` + `assert`, aucune dépendance externe).
- Zéro commentaire : le code doit être auto-explicite.
