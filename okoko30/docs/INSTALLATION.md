# Installation et exploitation

## Sur l’ordinateur de développement

Go 1.24 minimum (validation effectuée avec Go 1.27.3), Node.js 20.19 ou supérieur (validation avec Node 24), npm et un navigateur Chromium pour les tests visuels. Les fichiers statiques précompilés sont inclus ; Go peut donc compiler le backend sans Node si le frontend ne change pas.

1. Décompresser l’archive dans un nouveau dossier. Conserver l’ancienne installation.
2. Lancer `01-TESTER.cmd` : installation par `npm ci`, compilation du frontend, vérification des modules, analyse et tests Go.
3. Lancer `02-COMPILER-ARM64.cmd` pour obtenir `build/okoko30-linux-arm64` et l’outil de migration. La variable CGO est désactivée pour ARM64.
4. `03-DEMARRER-LOCAL.cmd` démarre une installation locale sur le port 8888 avec la configuration d’exemple.

Si Windows bloque les scripts téléchargés, les examiner puis autoriser leur exécution selon la politique de votre poste. Les scripts ne changent pas la politique d’exécution de Windows.

Pour les tests Playwright : lancer l’application locale, puis dans `frontend/`, exécuter `npx playwright install chromium` une seule fois et `npm run test:browser`. `CHROMIUM_PATH` permet d’utiliser un navigateur déjà installé. `OKOKO_REAL_BACKEND=1` active aussi les vérifications HTTP non simulées, à utiliser avec une base temporaire et une chaudière non configurée. Les autres tests utilisent des réponses simulées explicitement identifiées, exclusivement dans le navigateur de test.

## Configuration privée de la chaudière

Copier `backend/config.example.json` en configuration locale non versionnée, ou conserver le fichier de configuration déjà présent sur le routeur. Les noms de champs de la version 2.0 restent compatibles : `listen`, `boiler_url`, `database`, `poll_seconds`, `retention_days`, `csv_dir`, `import_minutes`, `backup_dir`.

Recommandation : laisser `boiler_url` vide dans le JSON et enregistrer l’URL complète dans un fichier privé extérieur aux sources. Sous Linux/OpenWrt :

```sh
mkdir -p /etc/okoko
chmod 700 /etc/okoko
# Créer /etc/okoko/boiler-url avec votre éditeur local ; ne pas le committer.
chmod 600 /etc/okoko/boiler-url
export OKOKO_BOILER_URL_FILE=/etc/okoko/boiler-url
/root/chaudiere-go -config /root/config.json
```

Ne pas placer l’URL secrète dans une commande enregistrée dans l’historique du shell. Une variable d’environnement privée `OKOKO_BOILER_URL` est aussi disponible. Le fichier référencé par `OKOKO_BOILER_URL_FILE` prend priorité. En cas d’erreur réseau, l’API n’expose pas l’URL ; les nouveaux journaux d’import masquent son chemin d’accès. Aucun accès réel à votre chaudière n’est requis pour développer l’interface.

Le chemin SQLite doit désigner la base existante, sans la remplacer. Les répertoires parents doivent exister et être accessibles en écriture. Le champ facultatif `timezone` vaut `Europe/Paris` par défaut. Les nouvelles dates du journal sont enregistrées en UTC avec un fuseau explicite, puis affichées à l’heure de Paris (été/hiver automatique). Les anciens imports sans fuseau sont interprétés en UTC, conformément au décalage constaté de 07:50 vers 09:50. `legacy_import_timezone` permet de préciser un autre fuseau source si nécessaire, par exemple `Europe/Paris` pour des anciens journaux déjà en heure locale. Cette conversion ne réécrit aucune ligne SQLite. Les dates des mesures CSV suivent le calendrier de l’installation. Les règles de fuseau sont embarquées dans le binaire, sans paquet de zones supplémentaire à installer sur OpenWrt.

## Sur OpenWrt ARM64

Le routeur exécute uniquement le binaire Go. Aucun Node.js, npm, frontend dev server ou accès à un CDN n’est nécessaire. Les polices, icônes, styles et graphiques sont embarqués. Les fichiers JS/CSS disposent de versions gzip précompilées.

Préparer SSH/SCP, `sqlite3`, `sha256sum` et `wget` sur le routeur pour le déploiement et la sauvegarde. Ne pas télécharger de base vide à la place de la base existante.

```powershell
.\scripts\Deploy.ps1 -Router ADRESSE_DU_ROUTEUR
```

Chemins par défaut compatibles avec l’ancien script : binaire `/root/chaudiere-go`, configuration `/root/config.json`, base `/root/chaudiere/chaudiere.db`, port 8888. Utiliser `-AppPath`, `-ConfigPath`, `-DatabasePath` et `-Port` si l’installation diffère. Le port doit correspondre à `listen` dans la configuration. Si le service procd `/etc/init.d/okoko` existe, il est arrêté et redémarré sans être réécrit. Sinon le programme est démarré directement ; aucun nouveau service de démarrage automatique n’est installé par ce script.

Le script vérifie le SHA-256 du binaire transféré, arrête l’ancienne instance, sauvegarde SQLite et contrôle la sauvegarde avec `PRAGMA quick_check`. Il conserve la configuration privée uniquement sur le routeur. Il vérifie `/health` puis `/api/v3/snapshot`. Ces contrôles établissent la disponibilité de l’application, pas la qualité de mesures réelles ; les vérifier ensuite à l’écran.

## Retour arrière

Le dossier `/root/okoko-rollback-HORODATAGE` annoncé à la fin du déploiement contient l’ancien binaire, la configuration et la sauvegarde SQLite. Pour restaurer le binaire :

```powershell
.\scripts\Rollback.ps1 -Router ADRESSE_DU_ROUTEUR -BackupDir /root/okoko-rollback-HORODATAGE
```

La base actuelle reste conservée. Une restauration manuelle de base est une opération distincte : arrêter le service, faire d’abord une sauvegarde de la base actuelle et contrôler le choix du fichier. Les sources de la version précédente sont disponibles dans `previous/okoko20/`, avec un exemple de configuration nettoyé ; l’archive d’origine demeure votre autre point de retour.

## NAS et sauvegardes

Le NAS reste facultatif. La collecte JSON et la navigation fonctionnent sans NAS. Les imports réseau sont traités en arrière-plan ; les requêtes de supervision ne font pas de `stat` ou de lecture de fichiers sur le NAS. Le statut « montage détecté » ne prétend pas vérifier la santé d’un serveur NFS distant. Un montage bloqué au niveau du système peut immobiliser le travailleur d’import : configurer les délais NFS côté OpenWrt. Les API d’historique ont des délais bornés et le cache temps réel reste accessible.

La page Système télécharge un ZIP SQLite cohérent, y compris les modifications WAL, vérifié avant l’envoi. Cette sauvegarde contient vos données historiques et doit rester privée. Aucun fichier de configuration de chaudière n’est inclus. L’ancienne fonction de sauvegarde programmée était désactivée ; aucune sauvegarde automatique nouvelle n’est annoncée.
