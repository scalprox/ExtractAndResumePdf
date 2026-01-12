#!/usr/bin/env python3
"""
Script de test pour l'OCR amélioré.
Usage: python test_ocr.py <chemin_image>
"""

import json
import sys
from pathlib import Path

import numpy as np
from PIL import Image
from paddleocr import PaddleOCR

# Importer les fonctions du service
sys.path.insert(0, str(Path(__file__).parent))

# Essayer d'importer depuis v2, sinon depuis l'original
from ocr_service import (
    group_lines_into_blocks,
    sort_blocks,
    merge_block_text,
    calculate_avg_confidence
)

print("✓ Utilisation de ocr_service_v2.py")


def test_ocr(image_path: str, show_detailed: bool = False, min_confidence: float = 0.75, eps_multiplier: float = 1.5):
    """Test l'OCR sur une image."""
    print(f"🔍 Traitement de l'image: {image_path}")
    print(f"⚙️  Paramètres: min_confidence={min_confidence}, eps_multiplier={eps_multiplier}")
    print("-" * 80)

    # Initialiser PaddleOCR
    ocr = PaddleOCR(use_angle_cls=True, lang="fr")

    # Charger l'image
    image = Image.open(image_path).convert("RGB")
    image_np = np.array(image)

    print(f"📐 Dimensions de l'image: {image.size[0]}x{image.size[1]}")
    print()

    # Effectuer l'OCR
    print("🤖 Exécution de l'OCR...")
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

    print(f"✅ {len(lines)} lignes détectées")
    print()

    # Regrouper en blocs
    print("📦 Regroupement en blocs...")
    blocks = group_lines_into_blocks(lines, eps_multiplier=eps_multiplier, min_confidence=min_confidence)
    blocks = sort_blocks(blocks)

    print(f"✅ {len(blocks)} blocs créés (après filtrage confiance >= {min_confidence})")
    print()

    # Afficher les résultats
    print("=" * 80)
    print("RÉSULTATS")
    print("=" * 80)
    print()

    output_blocks = []
    for idx, block in enumerate(blocks, 1):
        merged_text = merge_block_text(block)
        avg_conf = calculate_avg_confidence(block)

        print(f"📄 BLOC {idx} (confiance: {avg_conf:.1%}, {len(block)} lignes)")
        print("-" * 80)
        print(merged_text)
        print()

        if show_detailed:
            print("  Lignes individuelles:")
            for line in block:
                print(f"    • {line['text']} (conf: {line['confidence']:.1%})")
            print()

        output_blocks.append({
            "text": merged_text,
            "confidence": round(avg_conf, 3)
        })

    # Sauvegarder en JSON
    output_file = Path(image_path).stem + "_ocr_result.json"
    with open(output_file, 'w', encoding='utf-8') as f:
        json.dump({"blocks": output_blocks}, f, ensure_ascii=False, indent=2)

    print("=" * 80)
    print(f"💾 Résultat sauvegardé dans: {output_file}")
    print(f"📊 Nombre de tokens approximatifs: {sum(len(b['text'].split()) for b in output_blocks)}")


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python test_ocr.py <chemin_image> [options]")
        print()
        print("Options:")
        print("  --detailed              Afficher les lignes individuelles de chaque bloc")
        print("  --confidence <0.0-1.0>  Confiance minimale (défaut: 0.75)")
        print("  --epsilon <1.0-3.0>     Multiplicateur pour le regroupement (défaut: 1.5)")
        print()
        print("Exemples:")
        print("  python test_ocr.py image.jpg")
        print("  python test_ocr.py image.jpg --detailed")
        print("  python test_ocr.py image.jpg --confidence 0.8 --epsilon 2.5")
        sys.exit(1)

    image_path = sys.argv[1]
    show_detailed = "--detailed" in sys.argv

    # Parser les paramètres
    min_confidence = 0.75
    eps_multiplier = 1.5

    try:
        if "--confidence" in sys.argv:
            idx = sys.argv.index("--confidence")
            min_confidence = float(sys.argv[idx + 1])
    except (IndexError, ValueError):
        print("⚠️  Valeur invalide pour --confidence, utilisation de 0.75")

    try:
        if "--epsilon" in sys.argv:
            idx = sys.argv.index("--epsilon")
            eps_multiplier = float(sys.argv[idx + 1])
    except (IndexError, ValueError):
        print("⚠️  Valeur invalide pour --epsilon, utilisation de 1.5")

    if not Path(image_path).exists():
        print(f"❌ Erreur: L'image '{image_path}' n'existe pas")
        sys.exit(1)

    test_ocr(image_path, show_detailed, min_confidence, eps_multiplier)
