# Sources de retour arrière

`okoko20/` contient les sources Go de l’archive fournie, sans les binaires d’origine et sans son identifiant de connexion. Les tests originaux et les fichiers historiques sont conservés. Son exemple de configuration est volontairement nettoyé ; utiliser votre configuration privée sur le routeur.

Le déploiement 3.0 conserve aussi le binaire réellement installé avant remplacement et une sauvegarde cohérente de SQLite dans un dossier de retour arrière sur le routeur. Cette sauvegarde ne fait pas partie de la livraison et ne doit pas être copiée dans les sources. La restauration standard remet le binaire précédent sans remplacer la base actuelle.
