from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from typing import List

from app.services.embedding_service import embed_text, embed_batch

router = APIRouter()


class EmbedRequest(BaseModel):
    text: str


class EmbedBatchRequest(BaseModel):
    texts: List[str]


class EmbedResponse(BaseModel):
    embedding: List[float]
    dimensions: int


class EmbedBatchResponse(BaseModel):
    embeddings: List[List[float]]
    dimensions: int


@router.post("/embeddings/text", response_model=EmbedResponse)
async def embed_single(req: EmbedRequest):
    """Embed a single text string. Returns a 384-dim vector."""
    try:
        emb = embed_text(req.text)
        return EmbedResponse(embedding=emb, dimensions=len(emb))
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/embeddings/batch", response_model=EmbedBatchResponse)
async def embed_batch_endpoint(req: EmbedBatchRequest):
    """Embed multiple texts in a single call (max 100)."""
    if len(req.texts) > 100:
        raise HTTPException(status_code=400, detail="max 100 texts per batch")
    try:
        embs = embed_batch(req.texts)
        dims = len(embs[0]) if embs else 384
        return EmbedBatchResponse(embeddings=embs, dimensions=dims)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
