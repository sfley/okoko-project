# Lanceur Windows autonome avec reprise

Remplacer uniquement `INSTALLER-OKOKO.cmd` dans le dossier OKOKO existant, puis le lancer. Le dossier doit contenir `frontend/`, `backend/` et `scripts/remote-deploy.sh`. Aucun nouveau ZIP de l’application n’est nécessaire pour cette modification.

Le lanceur demande l’adresse du routeur. Une réponse vide prépare seulement les fichiers. L’adresse peut aussi être passée directement : `INSTALLER-OKOKO.cmd ADRESSE_DU_ROUTEUR`. `INSTALLER-OKOKO.cmd --prepare` prépare les binaires sans question ni déploiement.

- Déblocage des fichiers avec `Unblock-File`, sans changer `RemoteSigned`.
- Tests frontend, compilation frontend, vérification et tests Go, compilation ARM64.
- Étapes réussies mémorisées dans `test-results/install-resume.json`. Les empreintes des sources, versions des outils et résultats sont contrôlées avant de les ignorer. Une étape échouée n’est jamais enregistrée comme réussie.
- Déploiement ignoré seulement si le binaire actif du routeur est identique et que les API de santé répondent. Sinon, sauvegarde et déploiement via le script distant existant.
- Arrêt dès une erreur. PowerShell reste ouvert grâce à `-NoExit`, après succès comme après échec. Fermer la fenêtre reste une action manuelle.

Le fichier CMD contient sa propre logique PowerShell ; il ne charge pas les anciens modules `.ps1`. Il est distribué en ASCII avec fins de ligne CRLF, y compris en téléchargement direct depuis GitHub. Aucun mot de passe n’est enregistré. Le cache de reprise est exclu des sources et des livraisons.

## Vérification exécutée

Sous PowerShell 7.5.4/Linux : 12 contrôles réussis avec outils et déploiement simulés. Ils couvrent la reprise sans travail répété, les changements de sources, les fichiers manquants, un binaire corrompu, la reprise après échec, l’arrêt sur erreur SSH et le saut d’un déploiement déjà vérifié.

Une préparation réelle a aussi exécuté npm ci, les 5 tests frontend, le build frontend, go mod verify, go vet, les 20 tests Go et les deux compilations ARM64. La relance réelle a sauté toutes les étapes déjà validées. Seul le déblocage Windows a été simulé pour cette préparation Linux ; aucun déploiement réel effectué.

L’ouverture du `.cmd`, la fenêtre persistante et `Unblock-File` sous Windows ne sont pas présentés comme testés sur un poste Windows.
