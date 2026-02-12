- faire une application web (dans la techno de votre choix)
- créer une image docker de dev et un docker compose pour lancer votre application
- créer une image docker de prod dans le meme dockerfile avec le multi-stage build
- déployer votre application sur gcloud en utilisant cloud run
- déployer votre application en cd avec gitlab ci
- l'application doit:
    - afficher un qr code qui redirige vers votre application
    - afficher un slider d'images qui sont upload a travers votre site
    - sur chaque image on voit le nom de la personne qui l'a upload
    - a l'upload d'une image, une fonction cloud function doit cropper l'image pour qu'elles fassent toujours la meme taille
    - quand une nouvelle image est prete a etre affichée, elle est visible dans le slider sans actualiser

le plan de l'app:

app principale (flask):
/home -> affiche les tourniqué d'image (code js qui refresh pour afficher les nouvelles images) et le qr code

/upload -> formulaire d'upload de la photo avec un champs texte obligatoire pour le nom

=> trouver un moyen de link a la volé le nom de l'image et le nom de la personne upload (dict python ?)
script cloud function: 
- Code python qui prend une image en entrer et la crop pour s'assurer de la bonne taille
