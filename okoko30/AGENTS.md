# Livraisons Windows

- Fournir un seul point d'entrée utilisateur : `INSTALLER-OKOKO.cmd`, qui enchaîne déblocage, vérification des outils, tests, compilation et déploiement si un routeur est indiqué.
- Respecter la stratégie PowerShell `RemoteSigned` : débloquer les fichiers avec `Unblock-File` avant de charger un `.ps1`. Ne pas appeler `Set-ExecutionPolicy`, ni imposer `Bypass`.
- Les scripts internes sont des modules du lanceur, pas des étapes que l'utilisateur doit exécuter séparément. Arrêter la chaîne à la première erreur et éviter les tests ou compilations en double.
- Utiliser `npm.cmd` sous Windows et UTF-8 avec BOM pour les `.ps1` compatibles Windows PowerShell 5.1. Contrôler leur syntaxe ; signaler les tests Windows ou matériels non exécutés.
- Conserver la configuration privée, les données SQLite, les recharges et le retour arrière. Aucun mot de passe dans les sources.
