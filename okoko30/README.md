# OKOKO 3.0.1 Pro

Application locale de supervision d’une chaudière ÖkoFEN Pellematic PE CONDENS.
Interface React indépendante, compilée et intégrée au binaire Go. SQLite, collecte JSON et import CSV NAS conservés du socle fourni `okoko20.zip`.

Les corrections de cette livraison sont détaillées dans `docs/CHANGEMENTS-3.0.1.md`.

## Organisation

- `frontend/` : composants React, graphiques, cockpit et sources de styles.
- `backend/` : API HTTP, collecte, SQLite, import CSV et frontend compilé dans `internal/web/dist/`.
- `scripts/` : test, compilation ARM64, lancement Windows, déploiement SSH et retour arrière.
- `previous/okoko20/` : sources originales de la version précédente, avec configuration d’exemple nettoyée.
- `docs/INSTALLATION.md` : installation, configuration privée et fonctionnement sur OpenWrt.
- `docs/TESTS.md` : résultats réellement obtenus et limites.
- `docs/ARCHITECTURE.md` : API et choix de conservation des données.

## Démarrage de développement

Prérequis sur l’ordinateur : Go ≥ 1.24, Node.js ≥ 20.19, npm. Aucun Node.js sur le routeur.

```sh
cd frontend
npm ci
npm run build
cd ../backend
mkdir -p data
go test ./...
go run ./cmd/chaudiere -config config.example.json
```

L’application écoute par défaut sur le port 8888. Sans URL privée, elle démarre avec des états absents et les archives historiques déjà fournies par la version 2.0. Elle ne fabrique aucune télémétrie. Les données de test se trouvent uniquement dans `frontend/tests/` et les tests Go ; elles ne sont pas intégrées à l’application.

Sous Windows, les lanceurs `01-TESTER.cmd`, `02-COMPILER-ARM64.cmd`, `03-DEMARRER-LOCAL.cmd` et `04-DEPLOYER.cmd` appellent les scripts PowerShell correspondants. Le déploiement exige l’adresse du routeur : `04-DEPLOYER.cmd -Router ADRESSE_DU_ROUTEUR`.

## Aucun mot de passe dans les sources

Le fichier `backend/config.example.json` laisse `boiler_url` vide. Une URL de chaudière peut contenir un identifiant dans son chemin : elle doit rester privée. Placez-la dans un fichier extérieur au dépôt, protégé par les droits du système, et définissez `OKOKO_BOILER_URL_FILE` avec son chemin. `OKOKO_BOILER_URL` est également pris en charge. Les configurations locales, bases et fichiers `.env` sont exclus de Git et de l’archive de livraison.

Les scripts SSH utilisent votre authentification existante et la vérification habituelle de la clé du serveur. Ils ne contiennent aucun mot de passe.

## Données et retour arrière

Les formules de consommation n’ont pas été changées. Les recharges, corrections mensuelles et archives restent en SQLite. Une correction de conservation au démarrage empêche l’initialisation des archives de remplacer une valeur mensuelle déjà présente ; elle ne recalcule aucune consommation.

Le script de déploiement sauvegarde le binaire précédent, la configuration privée sur le routeur et une copie SQLite cohérente. Il remet le binaire précédent si les contrôles HTTP échouent. Aucun déploiement réel sur votre routeur n’a été exécuté dans cet environnement.
