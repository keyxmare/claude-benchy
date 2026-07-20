---
titre: Explorer les fichiers
public: tous
sources:
  - internal/server/browse.go
---

# Explorer les fichiers

> Choisir un chemin et inspecter un contenu sans quitter le navigateur, via des
> explorateurs servis côté serveur.

## User story

En tant qu'utilisateur, je veux choisir mes chemins (app, bundle, sortie…) et
vérifier le contenu d'un dossier ou d'un fichier depuis l'interface, afin de
configurer un banc sans manipuler de chemins absolus à la main.

## Ce qui est offert

- **Sélection des chemins** : chaque champ de chemin (app, bundle, dossier de
  sortie, configDir) offre un bouton 📁 qui ouvre un
  explorateur de dossiers **côté serveur** — le navigateur ne divulgue pas les
  chemins absolus, c'est donc benchy qui liste le système de fichiers de l'hôte.
- **Consulter les fichiers** : les champs app et bundle offrent un bouton 👁 qui
  ouvre une arborescence en lecture seule avec aperçu du contenu de chaque
  fichier — pour vérifier ce que contient un bundle ou l'app avant de lancer.
- **Aperçus intégrés au formulaire** : le champ App affiche le **premier niveau**
  du dossier de test (dossiers **et** fichiers), et chaque config affiche les
  entrées de premier niveau de son bundle en puces — via le listing en mode
  fichier, qui inclut les fichiers là où le mode dossier n'en montre pas.

Cette exploration parcourt librement le FS de l'hôte : c'est **assumé** par la
posture localhost (voir
[ADR-0008](../../architecture/decisions/0008-dashboard-stdlib-localhost.md)).

## Critères d'acceptation

- **Given** l'explorateur en mode dossier, **When** on liste un chemin, **Then**
  seuls les dossiers sont renvoyés (mode fichier : dossiers + fichiers réguliers).
- **Given** un chemin de fichier texte, **When** on l'ouvre, **Then** son contenu
  est servi en texte.
- **Given** un chemin de dossier passé à l'aperçu de fichier, **When** on le
  demande, **Then** il est refusé (404).
- **Given** un fichier trop volumineux ou binaire, **When** on l'ouvre, **Then**
  un message le signale plutôt que d'afficher le contenu.

## Cas non triviaux & limites

- Un chemin relatif ou absent retombe sur la racine ; l'aperçu de fichier exige un
  chemin absolu.
- La limite d'aperçu est de 512 Kio ; au-delà, un message remplace le contenu.
- Ces routes ne sont **pas** confinées à la racine (contrairement aux routes de
  résultats) — d'où l'importance de rester en localhost.

## Cas de test associés

`internal/server/server_test.go` : `TestBrowseListsDirectories`,
`TestFileServesTextAndRejectsDir`.
