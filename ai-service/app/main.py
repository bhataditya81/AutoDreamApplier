"""AutoDreamApplier AI Service - FastAPI application."""

from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.routers import resume, cover_letter, form_qa


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application startup and shutdown events."""
    # Startup
    print("AI Service starting up...")
    yield
    # Shutdown
    print("AI Service shutting down...")


app = FastAPI(
    title="AutoDreamApplier AI Service",
    description="Resume tailoring, cover letter generation, and form Q&A",
    version="0.1.0",
    lifespan=lifespan,
)

# CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include routers
app.include_router(resume.router, prefix="/api/v1/resume", tags=["resume"])
app.include_router(cover_letter.router, prefix="/api/v1/cover-letter", tags=["cover-letter"])
app.include_router(form_qa.router, prefix="/api/v1/form-qa", tags=["form-qa"])


@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {"status": "ok", "service": "ai-service", "version": "0.1.0"}
