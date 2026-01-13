# CrawlGameRules

**CrawlGameRules** est un outil automatisé permettant d'extraire, de numériser et de résumer le contenu de fichiers PDF (typiquement des règles de jeux de société) en utilisant l'OCR et l'intelligence artificielle.

## 📋 Présentation du projet

Le projet automatise le pipeline suivant :
1.  **Découpage PDF** : Extraction des images de chaque page d'un fichier PDF.
2.  **Traitement OCR** : Analyse de chaque image via un script Python (PaddleOCR) pour en extraire le texte brut.
3.  **Synthèse IA** : Combinaison des sorties OCR et génération d'un résumé détaillé des règles à l'aide d'un modèle de langage via **Ollama**.
4.  **Persistance** : Stockage du résultat final dans une base de données.

## Example

Vous pouvez retrouver un example [d'extraction du texte](./example/extraction.md) depuis [ce pdf](./example/notice_de_jeu.pdf)

## 🛠️ Prérequis

*   **Go** : Version 1.25 ou supérieure.
*   **Python** : Version 3.x avec `pip`.
*   **Base de données** : Configurée selon vos paramètres dans le fichier `.env`.

## 🚀 Installation

### 1. Service OCR (Python)
Installez les dépendances nécessaires pour le serveur de reconnaissance de texte :
```
bash
# Création de l'environnement virtuel
python -m venv .venv
source .venv/bin/activate  # Sur Windows : .venv\Scripts\activate

# Installation des paquets
pip install -r requirements.txt
```
### 2. Configuration
Assurez-vous d'avoir un fichier `.env` à la racine du projet contenant les variables nécessaires [voir le fichier example](.env.example).

## 🏃 Exécution

Pour faire fonctionner le projet, vous devez démarrer deux services distincts.

### Étape 1 : Démarrer le serveur OCR
Le programme Go communique avec un service Python pour l'OCR. Vous devez le lancer en premier :
```
bash
uvicorn ocr_service:app --host 0.0.0.0 --port 8000
```
*Le serveur tournera par défaut sur le port 8000.*

### Étape 2 : Lancer l'application Go
Une fois le service Python prêt, lancez le traitement principal :
```
bash
go run main.go
```
## 📂 Structure du code

*   `main.go` : Point d'entrée de l'application.
*   `ocr_service.py` : Serveur FastAPI utilisant PaddleOCR pour traiter les images.
*   `logic/` :
    *   `extract.go` : Logique d'extraction des images du PDF.
    *   `save.go` : Gestion de la sauvegarde en base de données.
