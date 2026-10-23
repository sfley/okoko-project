# Livraisons Windows

- Fournir un seul point d'entrée utilisateur : `INSTALLER-OKOKO.cmd`, qui enchaîne déblocage, vérification des outils, tests, compilation et déploiement si un routeur est indiqué.
- Respecter la stratégie PowerShell `RemoteSigned` : débloquer les fichiers avec `Unblock-File` avant de charger un `.ps1`. Ne pas appeler `Set-ExecutionPolicy`, ni imposer `Bypass`.
- Le parcours normal doit tenir dans ce lanceur autonome, sans demander une succession de scripts à exécuter. Arrêter la chaîne à la première erreur et éviter les tests ou compilations en double.
- Le lanceur doit rester téléchargeable seul et garder sa fenêtre ouverte, en particulier après une erreur. Reprendre les étapes déjà validées seulement si leurs sources et résultats n'ont pas changé ; contrôler le binaire actif sur le routeur avant de sauter un déploiement.
- Utiliser `npm.cmd` sous Windows et UTF-8 avec BOM pour les `.ps1` compatibles Windows PowerShell 5.1. Contrôler leur syntaxe ; signaler les tests Windows ou matériels non exécutés.
- Conserver la configuration privée, les données SQLite, les recharges et le retour arrière. Aucun mot de passe dans les sources.
