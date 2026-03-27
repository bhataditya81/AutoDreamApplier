# AutoDreamApplier — System Architecture

```mermaid
flowchart TB
    subgraph CLIENT["🌐 Client"]
        USER["👤 User Browser"]
    end

    subgraph VERCEL["▲ Vercel — autodreamapplier.vercel.app"]
        NEXT["Next.js 14\nApp Router"]
    end

    subgraph AUTH["🔐 Auth"]
        COGNITO["AWS Cognito\nRS256 JWT"]
        DEVAUTH["Dev HS256\nJWT (staging)"]
    end

    subgraph LAMBDA["⚡ AWS Lambda (us-east-1)"]
        APIGW["api-gateway\n/api/v1/*"]
        JOBDISC["job-discovery\nIndeed scraper"]
        JOBMATCH["job-matcher\nKeyword scorer"]
        FOLLOWUP["followup-scheduler\n1hr ticker"]
    end

    subgraph EC2["🖥️ EC2 t4g.nano ARM64 — 44.216.49.133"]
        APPLY["Apply Engine\n(Go + Asynq)"]
        BROWSER["Browser Pool\n(Chromium)"]
        AISERVICE["AI Service\n(Python FastAPI)"]
    end

    subgraph DATA["🗄️ Data Layer"]
        NEON[("Neon PostgreSQL\nusers · matches\napplications")]
        REDIS[("Upstash Redis\nAsynq queues\nrate limits · cache")]
        S3[("AWS S3\nresumes\nAI outputs\nscreenshots")]
    end

    subgraph EXTERNAL["🌍 External Services"]
        GEMINI["Gemini 1.5 Flash\nAI tailoring"]
        SES["AWS SES\nemail notifications"]
        BOARDS["Job Boards\nIndeed · Glassdoor"]
    end

    USER -->|HTTPS| NEXT
    NEXT -->|"proxy /api/*"| APIGW
    APIGW -->|validate| COGNITO
    APIGW -->|validate| DEVAUTH
    APIGW -->|read/write| NEON
    APIGW -->|enqueue TypeAIPrep| REDIS

    JOBDISC -->|scrape| BOARDS
    JOBDISC -->|insert jobs| NEON

    JOBMATCH -->|score + filter| NEON
    JOBMATCH -->|insert matches| NEON

    FOLLOWUP -->|check pending| NEON
    FOLLOWUP -->|send| SES

    %% 2-stage async pipeline
    REDIS -->|"① dequeue TypeAIPrep\n(weight 6)"| APPLY
    APPLY -->|tailor resume\ncover letter| AISERVICE
    AISERVICE -->|Gemini API| GEMINI
    AISERVICE -->|store outputs| S3
    APPLY -->|"② enqueue TypeBrowserApply\n(weight 3)"| REDIS
    REDIS -->|dequeue TypeBrowserApply| APPLY
    APPLY -->|fill ATS form| BROWSER
    APPLY -->|update status| NEON
    APPLY -->|notify| SES

    APIGW -->|read resumes| S3
    NEON -.->|connection pool| LAMBDA
    NEON -.->|connection pool| EC2

    classDef lambda fill:#FF9900,stroke:#FF9900,color:#000
    classDef ec2 fill:#1A73E8,stroke:#1A73E8,color:#fff
    classDef data fill:#2D6A4F,stroke:#2D6A4F,color:#fff
    classDef external fill:#6B35A8,stroke:#6B35A8,color:#fff
    classDef vercel fill:#000,stroke:#fff,color:#fff

    class APIGW,JOBDISC,JOBMATCH,FOLLOWUP lambda
    class APPLY,BROWSER,AISERVICE ec2
    class NEON,REDIS,S3 data
    class GEMINI,SES,BOARDS external
    class NEXT vercel
```

## Layer Summary

| Layer | Technology | Hosting |
|-------|-----------|---------|
| Frontend | Next.js 14 App Router | Vercel |
| Auth | AWS Cognito (RS256) + Dev HS256 | AWS |
| API | Go (Chi router) | AWS Lambda |
| Job Discovery | Go + Indeed scraper | AWS Lambda |
| Job Matcher | Go + keyword scorer | AWS Lambda |
| Apply Engine | Go + Asynq workers | EC2 t4g.nano |
| Browser Pool | Chromium (headless) | EC2 t4g.nano |
| AI Service | Python FastAPI + Gemini | EC2 t4g.nano |
| Database | PostgreSQL | Neon (serverless) |
| Queue / Cache | Redis | Upstash |
| Object Storage | S3 | AWS |
| Email | SES | AWS |

## 2-Stage Async Pipeline

```
User approves match
      │
      ▼
Lambda enqueues TypeAIPrep ──► Upstash Redis (QueueAIPrep weight:6)
                                      │
                                      ▼
                              Apply Engine dequeues
                                      │
                                      ▼
                              AI Service (Gemini)
                              ├─ Tailor resume
                              └─ Generate cover letter
                                      │
                                      ▼
                              Store outputs in S3
                                      │
                                      ▼
                        Enqueue TypeBrowserApply ──► Redis (QueueBrowserApply weight:3)
                                      │
                                      ▼
                              Apply Engine dequeues
                                      │
                                      ▼
                              Browser Pool (Chromium)
                              └─ Fill ATS form + submit
                                      │
                                      ▼
                              Update DB status → notify via SES
```
