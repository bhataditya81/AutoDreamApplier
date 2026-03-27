"""AutoDreamApplier AI Service — FastAPI entry point."""
import uvicorn
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.config import settings
from app.routes import health, resume, cover_letter, form_qa, embeddings

app = FastAPI(
    title="AutoDreamApplier AI Service",
    version="1.0.0",
    docs_url="/docs" if settings.app_env != "production" else None,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(health.router)
app.include_router(resume.router, prefix="/api/v1")
app.include_router(cover_letter.router, prefix="/api/v1")
app.include_router(form_qa.router, prefix="/api/v1")
app.include_router(embeddings.router, prefix="/api/v1")

if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0", port=8085, reload=settings.app_env != "production")
