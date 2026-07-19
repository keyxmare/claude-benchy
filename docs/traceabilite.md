---
titre: Matrice de traçabilité
public: dev
---

# Matrice de traçabilité

> Relie chaque fonctionnalité à sa user story, ses critères d'acceptation, ses
> tests (`*_test.go`, noms exacts) et son code. De quoi reconstruire le projet à
> l'identique. Les noms de tests sont copiés tels quels depuis les `*_test.go`.

## Contexte banc

| Fonctionnalité | User story | Critères d'acceptation | Cas de test (`*_test.go`) | Code |
|---|---|---|---|---|
| [Définir un banc](fonctionnel/banc/definir-un-banc.md) | Décrire quoi comparer d'une commande | Défauts appliqués ; chemins absolutisés ; banc invalide rejeté ; prompt hérité/exclusif ; retries tri-état | `TestLoadDefaultsAndOverrides`, `TestLoadRetries`, `TestLoadPromptFile`, `TestLoadErrors`, `TestLoadMissingBundle`, `TestLoadEvaluate`, `TestLoadEvaluateInvalidCheck` | `internal/spec/spec.go`, `internal/spec/paths.go`, `cmd/benchy/main.go` |
| Localisation des creds OAuth | Monter des creds à jour dans le sandbox | Export Trousseau sur darwin ; repli sur fichier existant ; jamais de Trousseau hors darwin | `TestResolveCredsFileDarwinExportsKeychain`, `TestResolveCredsFileDarwinKeychainMissKeepsExistingFile`, `TestResolveCredsFileNonDarwinNeverReadsKeychain` | `internal/spec/creds.go` |
| [Bundle & config de base](fonctionnel/banc/bundle-et-config-de-base.md) | Superposer et choisir remplacer/compléter | Remplace par défaut ; append si `keepBaseConfig` ; verbatim si app sans `CLAUDE.md` ; autres fichiers ajoutés | `TestPrepareOverlaysAndCommits`, `TestPrepareMergesClaudeMdWhenAsked`, `TestPrepareMergeCreatesClaudeMdWhenAppHasNone`, `TestPrepareGitAppUsesCommittedTree`, `TestFormCarriesKeepBaseConfig` | `internal/workspace/workspace.go` |

## Contexte exécution

| Fonctionnalité | User story | Critères d'acceptation | Cas de test (`*_test.go`) | Code |
|---|---|---|---|---|
| [Lancer et suivre](fonctionnel/execution/lancer-et-suivre.md) | Lancer d'une commande et suivre | Artefacts produits ; concurrence bornée ; échec enregistré non bloquant ; résumé « ✓ » | `TestRunEndToEndWithFake`, `TestRunRecordsPrepareFailure`, `TestCollectFilesReadsTouchedFilesOnly`, `TestPostImagePath` | `internal/runner/runner.go`, `cmd/benchy/main.go` |
| [Isolation sandbox](fonctionnel/execution/isolation-sandbox.md) | Environnement propre et isolé | `--rm` + montages minimaux ; flags Claude d'isolation ; pas de montage creds si absent | `TestRunArgs`, `TestRunArgsWithoutCreds`, `TestRunArgsWithEntrypoint`, `TestClaudeArgsForceNonInteractive`, `TestTailWriterKeepsLastBytesFlattened`, `TestTailWriterCapsRetainedBytes`, `TestTailWriterEmpty` | `internal/docker/docker.go`, `internal/runner/runner.go` |
| [Runs sans effet](fonctionnel/execution/runs-sans-effet.md) | Ne pas polluer les résultats | Détection dégénéré ; relance ≤ retries ; `0` → pas de relance ; exclusion des agrégations ; erreur dure non relancée | `TestRunRetriesDegenerateRunUntilItProducesWork`, `TestRunRecordsPersistentlyDegenerateRun`, `TestDegenerateAndOK`, `TestEvalByConfigAggregatesRunsAndExcludesDegenerate` | `internal/runner/runner.go`, `internal/report/report.go` |

## Contexte capture

| Fonctionnalité | User story | Critères d'acceptation | Cas de test (`*_test.go`) | Code |
|---|---|---|---|---|
| Parsing du transcript | Extraire les métriques d'un run | `result` obligatoire ; lignes parasites tolérées ; outils comptés depuis le flux | `TestParse`, `TestParseNoResult`, `TestParseIgnoresGarbageLines` | `internal/claude/stream.go` |
| Rendu lisible | Suivre la sortie de Claude | Rendu texte/outils/résultats ; troncature rune-safe ; live split/flush | `TestRenderAssistantTextAndTools`, `TestLiveWriterSplitsAndFlushes`, `TestTruncateRuneSafe` | `internal/claude/render.go` |
| Capture du diff | Mesurer le delta produit | Delta complet vs baseline ; aucun changement → patch vide | `TestCaptureChanges`, `TestCaptureNoChanges` | `internal/diffcap/diffcap.go` |

## Contexte évaluation

| Fonctionnalité | User story | Critères d'acceptation | Cas de test (`*_test.go`) | Code |
|---|---|---|---|---|
| [Checks & juge](fonctionnel/evaluation/checks-et-juge.md) | Évaluer l'adéquation à l'attendu | Checks rejoués + juge sur runs OK ; check échoué consigné ; dégénérés exclus ; JSON toléré ; modèle juge par défaut | `TestRunWithEvaluationJudgesAndChecks`, `TestRunEvaluationRecordsFailingCheck`, `TestBuildJudgePromptEmbedsOKRunsAndTruncates`, `TestParseJudgeToleratesFencesAndComputesScoreRanking`, `TestParseJudgeRejectsNonJSON`, `TestLoadEvaluate`, `TestLoadEvaluateInvalidCheck` | `internal/runner/judge.go`, `internal/runner/runner.go`, `internal/spec/spec.go` |
| [Comparaison & efficacité](fonctionnel/evaluation/comparaison-et-efficacite.md) | Comparer sur un même plan | Agrégation score moyen + dispersion ; pondération de niveau (respecté = 1, partiel = 0.5) ; exclusion dégénérés ; extrêmes désignés (best/mid/worst) ; « aucun run réussi » ; comparaison côte à côte | `TestAnalysisRanksAndRecommends`, `TestEvalByConfigAggregatesRunsAndExcludesDegenerate`, `TestEvalByConfigNilWithoutEvaluation`, `TestAnalysisWithNoSuccessfulRun`, `TestResultHTMLCollapsesSoftWraps`, `TestGolden`, `TestParseChanges`, `TestChangeSym`, `TestFileTreeHTML`, `TestNormalizeLevel`, `TestScoreFromCriteria`, `TestAggregateLevel`, `TestLevelWeight`, `TestLevelCSSClass`, `TestLevelSymbol`, `TestRankCSSClass`, `TestRankSymbol`, `TestEfficiencyRanksBestMidWorst`, `TestEfficiencyNoSuccessfulRun` | `internal/report/report.go`, `internal/report/templates.go` |
| [Re-générer un rapport](fonctionnel/evaluation/regenerer-un-rapport.md) | Régénérer sans relancer | Reconstruit depuis artefacts sans API ; verdict relu tel quel ; run = dossier avec `diff.patch` | `TestRunLaunchesJobAndStreamsToCompletion`, `TestPostImagePath`, `TestCollectFilesReadsTouchedFilesOnly` | `internal/runner/reload.go`, `cmd/benchy/main.go` |

## Contexte pilotage (dashboard web)

| Fonctionnalité | User story | Critères d'acceptation | Cas de test (`*_test.go`) | Code |
|---|---|---|---|---|
| [Dashboard web](fonctionnel/pilotage/dashboard-web.md) | Piloter depuis le navigateur | Formulaire rendu ; run + SSE jusqu'à `done` ; réaffichage sur erreur ; path traversal rejeté | `TestDashboardRenders`, `TestRunLaunchesJobAndStreamsToCompletion`, `TestRunRerendersFormOnValidationError`, `TestTranscriptRejectsPathTraversal`, `TestReportRejectsPathTraversal`, `TestFormSpecDropsBlankRowsAndParsesLists`, `TestFormRetriesRoundTrip`, `TestFormSpecRejectsInvalidNumber`, `TestChooseImagePrefersBenchImage`, `TestBenchDocMarshalsTidyYAML` | `internal/server/server.go`, `internal/server/jobs.go`, `internal/server/form.go`, `internal/server/history.go` |
| [Importer & reprendre](fonctionnel/pilotage/importer-et-reprendre.md) | Repartir d'un banc/run existant | Chemins réécrits en absolu ; reprise fidèle ; recours ancêtre/reconstruction ; erreurs rejetées | `TestNewImportsBenchFileWithAbsolutePaths`, `TestNewImportErrors`, `TestNewWithoutParamsRendersDefaultForm`, `TestNewFromInvalidDirIsRejected`, `TestResolveExistingFile`, `TestNewPrefillsFormFromRunConfig`, `TestReuseRecoversBundleFromAncestorBench`, `TestImportAbsolutizesRelativePaths`, `TestImportPreservesComments`, `TestImportLeavesEmptyAndConfigsNonSequenceUntouched`, `TestImportErrors`, `TestDocumentMapping`, `TestMapValue`, `TestAbsolutizeScalar` | `internal/server/reuse.go`, `internal/server/import.go`, `internal/spec/import.go` |
| [Appliquer & exporter](fonctionnel/pilotage/appliquer-et-exporter.md) | Adopter la sortie/config gagnante | Diff appliqué non committé ; échec 422 ; export accordé au nombre ; chemins invalides rejetés | `TestApplyAppliesPatchToProject`, `TestApplyErrors`, `TestConfigFilesLists`, `TestConfigFilesErrors`, `TestConfigFilesRejectsBadArtifact`, `TestExportConfigCopiesSelectedFiles`, `TestExportConfigSingleFileMessage`, `TestExportConfigErrors`, `TestApplyButtonOnlyWhenServed` | `internal/server/apply.go`, `internal/server/exportconfig.go` |
| [Explorer les fichiers](fonctionnel/pilotage/explorer-les-fichiers.md) | Choisir chemins et inspecter contenus | Listing dossiers/fichiers ; fichier texte servi ; dossier rejeté à l'aperçu | `TestBrowseListsDirectories`, `TestFileServesTextAndRejectsDir` | `internal/server/browse.go` |
