# Freedy — instructions projet

Projet Go : rendu 3D WebGPU, cible **fenêtre native** et **canvas HTML (WASM)**
avec une base de code unique. Précédence : **projet > profil > socle**.

## Toolchain — dérogation au socle

Le socle impose « tout runtime sous Docker ». **Ce projet déroge pour Go**, qui
tourne **sur l'hôte** :

- la fenêtre native WebGPU exige GLFW, un accès GPU et un serveur d'affichage ;
  la piloter depuis un conteneur imposerait du passthrough X11/GPU lourd et
  fragile, sans bénéfice ;
- Go est un compilateur, pas un service : aucun daemon résiduel sur l'hôte.

En conséquence :

- **`go build` / `go run` / `go test` / `go vet` : sur l'hôte.**
- **Le serveur web reste sous Docker** (`compose.yaml`, `make serve`) : c'est le
  seul composant « service » du projet.

## Cibles et build tags

- Séparation par build tag : `//go:build !js` (natif) et `//go:build js` (web).
- Backend GLFW natif choisi par tag : `NATIVE_TAGS=x11` (défaut) ou
  `NATIVE_TAGS=wayland`. Le fork GLFW compile sinon les deux backends et exige
  alors les headers Wayland.
- Toujours vérifier **les deux cibles** : `make check` lance `vet` en natif ET
  en `GOOS=js GOARCH=wasm`.

## Rendu

- API WebGPU commune via `github.com/oliverbestmann/webgpu`.
- Le code de rendu (`internal/renderer`) reste **agnostique de la plateforme** :
  il reçoit un `*wgpu.SurfaceDescriptor` et une taille, jamais de type GLFW ou
  `syscall/js`. Toute dépendance plateforme vit dans `cmd/freedy/main_*.go`.
- WGSL dans des fichiers `.wgsl` embarqués (`//go:embed`), pas de shader inline.

## Tests

Pour toute écriture ou modification de test Go dans ce projet, applique
impérativement la règle de test dédiée :

@.claude/rules/go-tests.md
