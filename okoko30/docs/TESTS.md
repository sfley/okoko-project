# Rapport de validation — OKOKO 3.0.1 Pro

Contrôles exécutés le 23 octobre 2026 dans l’environnement cloud Linux amd64 : Go 1.27.3, Node.js 24.19.0, npm 11.9.0, Chromium 154, Playwright 1.56.1.

## Résultats réellement obtenus

| Contrôle | Résultat et portée |
|---|---|
| `go test -json -count=1 ./...` | 20 tests réussis ; le serveur de test navigateur facultatif est sauté dans cette commande et exécuté séparément ci-dessous |
| `go test -race -count=1 ./...` | Réussi, aucune course détectée ; même serveur facultatif exclu |
| `go vet ./...` | Réussi |
| `go mod verify` | Tous les modules vérifiés ; `go.mod` et `go.sum` conservés |
| `npm test` | 5 tests réussis : valeurs absentes, calendrier invalide, absence de créneau inventé, minuit et unités du vent |
| `npm run build` | Frontend compilé et précompressé dans `backend/internal/web/dist/` |
| `OKOKO_REAL_BACKEND=1 OKOKO_RAW_FIXTURE=1 npm run test:browser` | 13 scénarios réussis, aucun échec ni saut |
| Serveur Go de test `TestBrowserFixtureServer` | Démarré séparément sur 127.0.0.1:8889 avec le JSON fourni ; scénario navigateur réussi, serveur arrêté et test terminé avec PASS |
| Linux ARM64, CGO désactivé | Application et outil d’import compilés ; format ELF aarch64 statique contrôlé, exécution sur routeur non effectuée |
| Linux amd64 | Application démarrée ; six routes SPA et API réellement ouvertes par Chromium |
| Conservation des fonctions | 7 fonctions identiques après normalisation Go : les deux calculs de consommation et cinq fonctions CSV ; empreintes dans `source-preservation.json` |
| Scripts shell | Syntaxe de `remote-deploy.sh`, `install-cloud.sh`, `start-cloud.sh` et `stop-cloud.sh` vérifiée avec `sh -n` ; arrêt et redémarrage local exécutés |
| Confidentialité de l’archive | Configurations d’exemple sans URL privée ; bases, configurations locales et caches exclus ; identifiant de connexion initial recherché sans l’afficher |

Les sorties Go, race, Playwright et les captures sont jointes dans ce dossier. Le code du serveur de test est dans un fichier `_test.go`, absent des binaires de production. Son calendrier est figé au 23 octobre 2026 pour reproduire l’extrait fourni. Aucun appel à une chaudière ou à un service météo réel n’est effectué par ce serveur.

## Vérifications des corrections

- **Horaires** : un import ancien `2026-10-23 07:50:00` est interprété en UTC et affiché à Paris à 09:50. Le même horaire en décembre donne 08:50. Un navigateur configuré en Asie/Tokyo affiche également les heures de Paris attendues. Les nouveaux imports enregistrent un instant UTC avec fuseau explicite. Les anciennes chaînes SQLite ne sont pas réécrites.
- **Menu** : une navigation principale unique de six liens ; les sections « Votre maison », « Votre installation » et le faux sélecteur « Ma maison » ont été retirés du menu.
- **Circuits** : `name` alimente les modules, la bibliothèque, les titres Chauffage, les légendes et l’activité. « Chambres » et « Plancher » sont vérifiés depuis le JSON fourni.
- **Températures brutes** : 196 → 19,6 °C, 206 → 20,6 °C, 397 → 39,7 °C. Les facteurs explicites restent respectés. Les capteurs déconnectés restent absents ; le stock n’est pas mis à l’échelle.
- **Météo** : les 25 chaînes directes `forecast.L_w_0` à `L_w_24` sont lues. Les dates françaises sans année sont rattachées à la collecte, y compris le changement d’année et le changement d’heure. L’extrait montre quatre jours ; lundi ne fournit que le matin, sans midi ou soir inventé. Lever 08:31, coucher 19:04, vent 57 km/h sans unité répétée.
- **Graphiques** : la série Extérieur figure dans les températures globales du cockpit et d’Analyse. HK1/HK2 ont un historique interactif de 24 heures entre consigne et pompe, dans le cockpit et Chauffage. Chaque page partage une seule requête d’historique entre ses graphiques.
- **Journal** : 25 imports parcourus sur trois pages de 10, 10 et 5 lignes, sans perte ni duplication ; navigation avant et arrière testée. Les imports déjà présents restent en base.

## Navigateur et inspection visuelle

Les six pages ont été ouvertes et inspectées sur ordinateur 1440 × 1050, tablette 1024 × 900 et mobile 390 × 844. Les captures sous `captures/` sont issues de Chromium réel. Les données synthétiques portent un bandeau ; une capture météo supplémentaire utilise l’extrait JSON fourni, également identifié.

Les scénarios contrôlent aussi le déplacement par glisser-déposer, le redimensionnement par poignée, l’ajout/retrait de modules, les positions et dimensions conservées après actualisation, les modules compacts, l’absence de débordement horizontal de la page, les légendes et périodes, le formulaire de recharge, les données absentes et les consignes techniques de 8 °C. Les tableaux larges disposent de leur propre défilement horizontal.

Un premier contrôle du cockpit comparait aussi des indicateurs internes transitoires de la bibliothèque de grille. Le contrôle a été recentré sur les identifiants, positions et dimensions réellement à conserver ; le scénario complet a ensuite réussi. Les captures de pagination incluent un contrôle mobile supplémentaire après stabilisation de la navigation responsive.

Les tests Go couvrent la conservation des recharges/corrections au redémarrage, les formules pellets, la déduplication CSV, la sauvegarde WAL cohérente, les valeurs NULL, le cache, les origines étrangères et le masquage des accès privés. Huit lectures successives des API snapshot et météo ne provoquent aucun appel chaudière supplémentaire.

## Limites restantes

- Aucun accès à votre chaudière réelle, NAS réel ou SQLite de production. Le JSON fourni est testé, la connexion matérielle après déploiement reste à vérifier.
- ARM64 compilé, pas exécuté sur votre routeur. Mémoire et performances OpenWrt non mesurées ici.
- Scripts PowerShell relus et inclus, pas exécutés sous Windows. Déploiement SSH et restauration non exécutés sur votre matériel.
- Les anciens imports sans fuseau sont interprétés en UTC conformément au décalage confirmé. Une installation ayant utilisé une autre horloge peut préciser `legacy_import_timezone` ; les dates illisibles restent signalées sans conversion arbitraire.
- Les compteurs historiques d’import sont ceux déjà enregistrés, sans reconstitution des statistiques manquantes.
- Le cockpit reste propre au navigateur. Aucun écran tactile physique ou Safari testé.
- Chauffage en supervision : aucune commande de chauffe réseau ajoutée.
- Le NAS reste facultatif ; un montage NFS bloqué dans le noyau peut immobiliser le travailleur d’import. Les vues temps réel n’accèdent pas au NAS. Un blocage matériel n’a pas été reproduit.
- Les détails météo dépendent des créneaux transmis. Les formats non reconnus restent visibles sans affectation arbitraire.
- La sauvegarde SQLite est déclenchée à la demande ; aucune nouvelle sauvegarde périodique annoncée.
