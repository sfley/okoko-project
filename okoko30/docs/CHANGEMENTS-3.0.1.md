# Correctif OKOKO 3.0.1 — 23 octobre 2026

- Horaires des imports convertis d’UTC vers Europe/Paris avec gestion été/hiver ; conservation des anciennes chaînes SQLite.
- Navigation unifiée, sans les catégories « Ma maison » et « Votre installation ».
- Noms des circuits ÖkoFEN repris dans le cockpit, Chauffage, les légendes, la bibliothèque et l’activité.
- Lecture du format JSON brut en dixièmes de degré, ainsi que du format à métadonnées avec facteur explicite.
- Météo issue des chaînes directes `forecast.L_w_*`, calendrier français sans année et unités du vent corrigés.
- Température extérieure ajoutée aux graphiques globaux.
- Mini-historiques HK1/HK2 entre consigne et pompe dans le cockpit et Chauffage, à partir des CSV existants, sans requête chaudière supplémentaire.
- Journal de 10 imports par page, accès à l’intégralité des anciens imports.

Aucun calcul de consommation modifié. Les dispositions personnalisées existantes, recharges et bases restent conservées. Aucun mot de passe dans les sources ou les exemples de configuration.

## Mise à jour depuis 3.0.0

Conserver votre configuration privée et le chemin de la base existante. Le déploiement remplace le binaire, pas vos données. Le fuseau d’affichage est Europe/Paris par défaut et l’ancien journal est interprété en UTC, sans ajout obligatoire à votre configuration actuelle. Les nouveaux champs facultatifs sont documentés dans `INSTALLATION.md`.

Le script de déploiement conserve le binaire précédent et une sauvegarde SQLite. La livraison 3.0.0 reste aussi disponible dans le commit Git `489d135e73303b78eb05b5d27059db11baae275e` de la branche `okoko-v3.0`. Ne pas restaurer une ancienne base à la place de données nouvelles pour simplement revenir au binaire précédent.

## Lanceur Windows unifié

`INSTALLER-OKOKO.cmd` remplace les quatre lanceurs numérotés. Il débloque les fichiers sans modifier `RemoteSigned`, vérifie les outils, enchaîne tests, compilation et déploiement si un routeur est indiqué. Les doublons d’exécution sont évités ; la première erreur arrête la chaîne. Les scripts PowerShell ont un encodage compatible Windows PowerShell 5.1 et les erreurs de syntaxe dues aux apostrophes typographiques ont été corrigées. Aucun changement de l’application, des calculs ou des données.
