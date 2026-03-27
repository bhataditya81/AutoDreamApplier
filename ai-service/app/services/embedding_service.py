"""
Embedding service using sentence-transformers all-MiniLM-L6-v2.
384-dimensional vectors, runs on CPU, ~20ms per text.
"""
from typing import List
import numpy as np

# Module-level singleton — loaded lazily on first use to avoid startup cost.
_model = None


def get_model():
    global _model
    if _model is None:
        from sentence_transformers import SentenceTransformer
        _model = SentenceTransformer("all-MiniLM-L6-v2")
    return _model


def embed_text(text: str) -> List[float]:
    """Embed a single text string. Returns 384-dim vector."""
    model = get_model()
    # Truncate to rough char limit (model has a 512-token window)
    text = text[:4096]
    embedding = model.encode(text, normalize_embeddings=True)
    return embedding.tolist()


def embed_batch(texts: List[str]) -> List[List[float]]:
    """Embed multiple texts efficiently."""
    model = get_model()
    texts = [t[:4096] for t in texts]
    embeddings = model.encode(texts, normalize_embeddings=True, batch_size=32)
    return embeddings.tolist()


def cosine_similarity(a: List[float], b: List[float]) -> float:
    """Cosine similarity between two normalized vectors."""
    a_arr = np.array(a)
    b_arr = np.array(b)
    return float(np.dot(a_arr, b_arr))
