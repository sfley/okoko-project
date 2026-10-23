# Architecture et conservation

## Frontend

React 18, Vite, composants réutilisables, icônes SVG Lucide, graphiques Recharts et grille React Grid Layout. Les sources se trouvent exclusivement dans `frontend/`. `npm ci` utilise le verrou de dépendances. `npm run build` produit `backend/internal/web/dist/` puis précompresse JS et CSS. Go embarque ces fichiers avec `embed`. La police Inter est locale ; aucun appel de télémétrie ou de CDN n’est utilisé par la version 3.0.

Le cockpit enregistre ses modules et les dispositions responsive dans `localStorage` sous `okoko30.cockpit.v1`, avec validation du format et gestion des erreurs de stockage. Déplacement par poignée, redimensionnement par coin, commande de taille compacte/détaillée et commandes de déplacement accessibles. Les dispositions ordinateur/mobile restent indépendantes. Elles sont propres au navigateur, sans synchronisation entre appareils. L’ancien stockage `okoko20-cockpit` est conservé et non écrasé.

Le frontend consulte le cache JSON toutes les 15 secondes, sans déclencher une requête chaudière. Les historiques et la météo sont relus à intervalle plus espacé ; les onglets masqués ne déclenchent pas de rafraîchissements périodiques. La navigation fonctionne par historique navigateur. Les pages secondaires sont chargées à la demande.

## Backend

Le code de collecte, le schéma SQLite, les imports CSV et journaux HTTP ainsi que les calculs viennent de l’archive fournie. Le backend courant ne contient plus d’interface HTML métier ; les vues originales et leurs tests restent dans `previous/okoko20/` pour le retour arrière. Une nouvelle API sert les composants :

| Méthode | Route | Fonction |
|---|---|---|
| GET | `/api/v3/snapshot` | Cache temps réel, fraîcheur, circuits et pellets ; aucun appel chaudière |
| GET | `/api/v3/history?days=1` | Mesures historiques réelles, échantillonnage temporel borné, NULL préservés |
| GET | `/api/v3/pellets` | Mois archivés/corrigés et liste de recharges existante |
| GET | `/api/v3/weather` | Données météo effectivement reçues de la chaudière |
| GET | `/api/v3/transitions` | 100 dernières transitions observées |
| GET | `/api/v3/system?page=1` | SQLite, montage NAS détecté et journal unique, 10 imports par page |
| POST | `/api/v3/refills` | Ajout de recharge, validation et détection de doublon conservées |
| POST | `/api/v3/import` | Import en arrière-plan, sans travail parallèle concurrent |
| GET | `/api/v3/backup` | ZIP de sauvegarde SQLite cohérente et contrôlée |
| GET | `/api/v3/export?type=measurements` | Export des mesures ; `monthly` et `daily` également disponibles |
| GET | `/health` | Disponibilité HTTP et SQLite |

La collecte conserve le délai minimum de 2,5 secondes entre requêtes chaudière et l’intervalle configuré. Les commandes de mode/consigne restent sur la régulation ÖkoFEN : l’ancienne application ne proposait pas d’API de commande documentée, aucune commande réseau n’a été inventée.

Les requêtes POST exigent du JSON et refusent les origines navigateur étrangères. Le programme reste destiné au réseau domestique de confiance ; il ne comporte pas d’authentification d’accès à l’interface. Les routes de debug de l’ancienne version ne sont pas exposées dans la version 3.0. Les réponses JSON et les fichiers statiques ne contiennent pas la configuration privée.

## Formules et données

Les fonctions `monthlyStockMethod` et `recalculateYear` sont identiques à celles de l’archive, après normalisation Go. Les fonctions de mapping et d’import CSV sont aussi conservées. Leur comparaison est enregistrée dans `docs/source-preservation.json`.

Une correction précise a été apportée à l’initialisation `seedHistoricalConsumption` : l’insertion des références historiques n’écrase plus un mois déjà présent. Il s’agit de préserver les corrections existantes au redémarrage, sans changement de formule. Les corrections de `monthly_overrides` sont appliquées à la lecture comme dans l’ancien export. Aucune suppression ou réécriture de recharge n’est déclenchée par la nouvelle interface. Les références historiques déjà incluses dans l’archive sont conservées comme telles.

Une valeur manquante apparaît comme `—`, jamais comme une mesure zéro. Les consignes HK1/HK2 suivent les valeurs programmées du mode actif, pas `L_roomtemp_set` technique. Les prévisions sont rattachées à une date uniquement si la source en fournit une explicite. Les créneaux matin/midi/soir n’apparaissent qu’avec un horaire source explicite. Les températures de test sont isolées des sources de production et signalées dans les captures.

## Formats ÖkoFEN et heures (3.0.1)

L’API d’affichage normalise les températures JSON brutes en dixièmes de degré ; lorsqu’un objet contient un facteur explicite, celui-ci fait autorité. Le moteur de consommation et le mapping CSV restent inchangés. Le champ `name` des circuits sert de libellé, sans changer les identifiants persistés du cockpit. Les mini-graphiques HK1/HK2 utilisent la requête d’historique CSV de la page.

Les chaînes météo directes et les objets à métadonnées sont acceptés. Une date française sans année est rattachée à l’année la plus proche de la collecte, uniquement à moins de 14 jours ; les dates complètes sont conservées. Les prévisions sont normalisées avec leur jour, heure et fuseau, sans appel météo externe. Un format inconnu est laissé visible et non affecté.

Le journal est paginé par identifiant décroissant. L’API renvoie la date d’origine et une date avec fuseau lorsque celle-ci est reconnue. Les nouveaux imports sont horodatés en UTC/RFC3339. Les anciennes dates sans fuseau sont interprétées en UTC par défaut, conformément au décalage confirmé, sans migration destructive de la base. `legacy_import_timezone` est une possibilité de correction pour une autre horloge source.
