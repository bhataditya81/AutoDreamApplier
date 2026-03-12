# Browser Pool Fleet (`cmd/browser-pool`)

The Browser Pool is a Node.js microservice designed to act as a physical automation fleet for AutoDreamApplier. It handles the most volatile portion of the entire ecosystem: Interacting with the raw DOM of ATS websites.

## 1. Architectural Role
The Go `Apply Engine` orchestration is brilliant at fault tolerance and state management, but controlling headless browsers natively in Go (via Chromedp) is often clunky.
By isolating the browser control into an ephemeral, stateless Node.js endpoint, it allows the system to leverage the **Playwright** ecosystem for unparalleled DOM reliability and anti-bot evasion.

Additionally, because browsers consume huge amounts of RAM, the `browser-pool` container can be deployed independently onto memory-optimized AWS EC2 Spot Instances while the core Go services live on cheap t3.micro instances.

## 2. API Contract & Stateless Execution
The Browser Pool maintains zero state. It does not speak to PostgreSQL or Redis. It solely exposes an internal HTTP API.

### `POST /apply`
The `apply-engine` sends a monolithic JSON tree containing absolute instructions.
```json
{
  "ats_type": "lever",
  "apply_url": "https://jobs.lever.co/company/jobId/apply",
  "user_data": {
    "first_name": "John",
    "last_name": "Doe",
    "email": "john@example.com",
    "linkedin": "https://linkedin.com/in/johndoe",
    "github": "https://github.com/johndoe",
    "s3_resume_key": "autodream-resumes/uuid-tailored.pdf"
  }
}
```

## 3. Automation Lifecycle

When the `POST /apply` route is hit:
1. **Asset Retrieval:** The Node process uses the `aws-sdk` to pull the physical PDF resume from S3/MinIO to a local `/tmp/` directory.
2. **Context Instantiation:** A new incognito Playwright Context is booted with normalized `User-Agent` strings.
3. **Plugin Routing:** The payload specifies an `ats_type`. The service dynamically loads the corresponding handler (e.g., `src/plugins/lever.ts`).
4. **DOM Navigation:**
   - Playwright navigates to the `apply_url`.
   - Native `locator.fill()` and `locator.setInputFiles()` commands physically map the JSON tree to the Lever/Greenhouse input boxes.
   - The "Submit Application" button is executed.
5. **Proof of Concept Verification:** Playwright waits for the "Success" confirmation page to render in the DOM.
6. **Screenshot Upload:** Playwright shoots a full-page photo of the confirmation screen. This image is immediately uploaded back to S3 (`autodream-screenshots/uuid-proof.png`).
7. **Cleanup:** Browser context is closed, `/tmp/` files are flushed, and an `HTTP 200 { "screenshot_url": "..." }` is returned to the Apply Engine.

## 4. Bot Evasion
Modern job boards deploy Cloudflare and DataDome to prevent spam applications. AutoDreamApplier utilizes `playwright-extra` and `puppeteer-extra-plugin-stealth` to scrub `webdriver` flags and spoof human navigation patterns, preventing automation blocks.
