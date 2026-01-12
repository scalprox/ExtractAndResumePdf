import io
from typing import List, Dict, Tuple

import numpy as np
from PIL import Image
from fastapi import FastAPI, UploadFile, File
from paddleocr import PaddleOCR
from sklearn.cluster import DBSCAN

app = FastAPI()

ocr = PaddleOCR(use_angle_cls=True, lang="fr")


def get_bbox_center(bbox: List[List[float]]) -> Tuple[float, float]:
    """Calcule le centre d'une bounding box."""
    x_coords = [pt[0] for pt in bbox]
    y_coords = [pt[1] for pt in bbox]
    return (sum(x_coords) / len(x_coords), sum(y_coords) / len(y_coords))


def get_bbox_bounds(bbox: List[List[float]]) -> Tuple[float, float, float, float]:
    """Retourne les limites (x_min, y_min, x_max, y_max) d'une bbox."""
    x_coords = [pt[0] for pt in bbox]
    y_coords = [pt[1] for pt in bbox]
    return (min(x_coords), min(y_coords), max(x_coords), max(y_coords))


def calculate_line_height(lines: List[Dict]) -> float:
    """Calcule la hauteur moyenne des lignes pour déterminer l'espacement."""
    if not lines:
        return 20.0

    heights = []
    for line in lines:
        if 'bbox' in line and line['bbox']:
            _, y_min, _, y_max = get_bbox_bounds(line['bbox'])
            heights.append(y_max - y_min)

    return np.median(heights) if heights else 20.0


def group_lines_into_blocks(lines: List[Dict], eps_multiplier: float = 1.5, min_confidence: float = 0.0) -> List[
    List[Dict]]:
    """
    Regroupe les lignes en blocs textuels basés sur la proximité spatiale.

    Args:
        lines: Liste de dictionnaires contenant 'text', 'confidence', 'bbox'
        eps_multiplier: Multiplicateur pour la distance d'epsilon (plus grand = blocs plus larges)

    Returns:
        Liste de blocs, chaque bloc contenant une liste de lignes
    """
    if not lines:
        return []

    # Calculer la hauteur moyenne des lignes pour adapter l'epsilon
    avg_line_height = calculate_line_height(lines)
    eps = avg_line_height * eps_multiplier

    # Préparer les données pour le clustering
    # On utilise le centre de chaque bbox
    centers = []
    for line in lines:
        if 'bbox' in line and line['bbox']:
            center_x, center_y = get_bbox_center(line['bbox'])
            # Donner plus de poids à la coordonnée Y (proximité verticale plus importante)
            centers.append([center_x * 0.3, center_y])

    if not centers:
        return [lines]

    centers = np.array(centers)

    # Clustering DBSCAN
    clustering = DBSCAN(eps=eps, min_samples=1, metric='euclidean').fit(centers)
    labels = clustering.labels_

    # Regrouper les lignes par cluster
    blocks = {}
    for idx, label in enumerate(labels):
        if label not in blocks:
            blocks[label] = []
        blocks[label].append(lines[idx])

    # Convertir en liste de blocs
    return list(blocks.values())


def sort_lines_in_block(lines: List[Dict]) -> List[Dict]:
    """
    Trie les lignes dans un bloc de haut en bas, gauche à droite.
    """
    if not lines:
        return lines

    # Trier par coordonnée Y (haut en bas), puis par X (gauche à droite)
    def sort_key(line):
        if 'bbox' in line and line['bbox']:
            x_min, y_min, _, _ = get_bbox_bounds(line['bbox'])
            return (y_min, x_min)
        return (0, 0)

    return sorted(lines, key=sort_key)


def sort_blocks(blocks: List[List[Dict]]) -> List[List[Dict]]:
    """
    Trie les blocs de haut en bas, gauche à droite.
    """
    if not blocks:
        return blocks

    def block_sort_key(block):
        if not block:
            return (0, 0)

        # Utiliser la position moyenne du bloc
        y_coords = []
        x_coords = []

        for line in block:
            if 'bbox' in line and line['bbox']:
                x_min, y_min, _, _ = get_bbox_bounds(line['bbox'])
                y_coords.append(y_min)
                x_coords.append(x_min)

        avg_y = sum(y_coords) / len(y_coords) if y_coords else 0
        avg_x = sum(x_coords) / len(x_coords) if x_coords else 0

        return (avg_y, avg_x)

    return sorted(blocks, key=block_sort_key)


def merge_block_text(block: List[Dict]) -> str:
    """
    Fusionne le texte d'un bloc en un seul string.
    """
    sorted_lines = sort_lines_in_block(block)
    return " ".join([line['text'] for line in sorted_lines if line['text'].strip()])


def calculate_avg_confidence(block: List[Dict]) -> float:
    """
    Calcule la confiance moyenne d'un bloc.
    """
    confidences = [line['confidence'] for line in block if line.get('confidence') is not None]
    return sum(confidences) / len(confidences) if confidences else 0.0


@app.post("/ocr")
async def ocr_page(file: UploadFile = File(...)):
    image_bytes = await file.read()
    image = Image.open(io.BytesIO(image_bytes)).convert("RGB")
    image_np = np.array(image)

    result = ocr.ocr(image_np)
    lines = []

    if result and len(result) > 0:
        ocr_result = result[0]

        texts = ocr_result.get('rec_texts', [])
        scores = ocr_result.get('rec_scores', [])
        polys = ocr_result.get('rec_polys', [])

        for i, text in enumerate(texts):
            if not text.strip():
                continue

            confidence = float(scores[i]) if i < len(scores) else None

            bbox_list = []
            if i < len(polys):
                poly = polys[i]
                bbox_list = [[float(pt[0]), float(pt[1])] for pt in poly]

            lines.append({
                "text": text,
                "confidence": confidence,
                "bbox": bbox_list
            })

    # Regrouper les lignes en blocs
    blocks = group_lines_into_blocks(lines, eps_multiplier=1.5)

    # Trier les blocs
    blocks = sort_blocks(blocks)

    # Formater la sortie de manière optimisée pour l'IA
    output_blocks = []
    for block in blocks:
        merged_text = merge_block_text(block)
        avg_conf = calculate_avg_confidence(block)

        if avg_conf < 0.7:
            continue

        output_blocks.append({
            "text": merged_text,
            "confidence": round(avg_conf, 3)
        })

    return {"blocks": output_blocks}


@app.post("/ocr/detailed")
async def ocr_page_detailed(file: UploadFile = File(...)):
    """
    Version détaillée qui retourne aussi les positions des blocs.
    Utile pour le debug ou si tu as besoin des coordonnées.
    """
    image_bytes = await file.read()
    image = Image.open(io.BytesIO(image_bytes)).convert("RGB")
    image_np = np.array(image)

    result = ocr.ocr(image_np)
    lines = []

    if result and len(result) > 0:
        ocr_result = result[0]

        texts = ocr_result.get('rec_texts', [])
        scores = ocr_result.get('rec_scores', [])
        polys = ocr_result.get('rec_polys', [])

        for i, text in enumerate(texts):
            if not text.strip():
                continue

            confidence = float(scores[i]) if i < len(scores) else None

            bbox_list = []
            if i < len(polys):
                poly = polys[i]
                bbox_list = [[float(pt[0]), float(pt[1])] for pt in poly]

            lines.append({
                "text": text,
                "confidence": confidence,
                "bbox": bbox_list
            })

    # Regrouper les lignes en blocs
    blocks = group_lines_into_blocks(lines, eps_multiplier=1.5)

    # Trier les blocs
    blocks = sort_blocks(blocks)

    # Formater la sortie détaillée
    output_blocks = []
    for block in blocks:
        merged_text = merge_block_text(block)
        avg_conf = calculate_avg_confidence(block)

        # Calculer la bounding box globale du bloc
        all_x = []
        all_y = []
        for line in block:
            if 'bbox' in line and line['bbox']:
                for pt in line['bbox']:
                    all_x.append(pt[0])
                    all_y.append(pt[1])

        block_bbox = None
        if all_x and all_y:
            block_bbox = {
                "x_min": min(all_x),
                "y_min": min(all_y),
                "x_max": max(all_x),
                "y_max": max(all_y)
            }

        output_blocks.append({
            "text": merged_text,
            "confidence": round(avg_conf, 3),
            "bbox": block_bbox,
            "line_count": len(block)
        })

    return {"blocks": output_blocks}
