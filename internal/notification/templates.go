package notification

import "html/template"

// All templates are parsed once at package init.
// Panics at startup if a template is malformed — this is intentional (fail fast).

var (
	tmplSubmitted   = template.Must(template.New("submitted").Parse(htmlSubmitted))
	tmplFailed      = template.Must(template.New("failed").Parse(htmlFailed))
	tmplOutcome     = template.Must(template.New("outcome").Parse(htmlOutcome))
	tmplWeeklyDigest = template.Must(template.New("weekly_digest").Parse(htmlWeeklyDigest))
)

// ─── Submitted ────────────────────────────────────────────────────────────────

const htmlSubmitted = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Application Submitted</title>
</head>
<body style="margin:0;padding:0;background:#f4f6f9;font-family:Inter,Helvetica,Arial,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="background:#f4f6f9;padding:40px 0;">
    <tr><td align="center">
      <table width="600" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 2px 8px rgba(0,0,0,.08);">
        <!-- Header -->
        <tr><td style="background:#4f46e5;padding:32px 40px;">
          <h1 style="margin:0;color:#ffffff;font-size:22px;font-weight:700;">AutoDreamApplier</h1>
        </td></tr>
        <!-- Body -->
        <tr><td style="padding:40px;">
          <p style="margin:0 0 8px;color:#374151;font-size:16px;">Hi {{.UserName}},</p>
          <h2 style="margin:16px 0;color:#111827;font-size:24px;">✅ Application submitted!</h2>
          <p style="color:#6b7280;font-size:15px;line-height:1.6;">
            We successfully applied to <strong>{{.JobTitle}}</strong> at <strong>{{.Company}}</strong> on your behalf.
          </p>
          <!-- Job card -->
          <table width="100%" style="background:#f9fafb;border:1px solid #e5e7eb;border-radius:8px;margin:24px 0;">
            <tr><td style="padding:20px;">
              <p style="margin:0 0 4px;font-size:13px;color:#9ca3af;text-transform:uppercase;letter-spacing:.05em;">Position</p>
              <p style="margin:0 0 16px;font-size:17px;font-weight:600;color:#111827;">{{.JobTitle}}</p>
              <p style="margin:0 0 4px;font-size:13px;color:#9ca3af;text-transform:uppercase;letter-spacing:.05em;">Company</p>
              <p style="margin:0 0 16px;font-size:17px;font-weight:600;color:#111827;">{{.Company}}</p>
              <p style="margin:0 0 4px;font-size:13px;color:#9ca3af;text-transform:uppercase;letter-spacing:.05em;">Application ID</p>
              <p style="margin:0;font-size:13px;color:#6b7280;font-family:monospace;">{{.ApplicationID}}</p>
            </td></tr>
          </table>
          <p style="color:#6b7280;font-size:14px;line-height:1.6;">
            We'll notify you when the employer views your application or requests an interview.
            You can track all your applications in your dashboard.
          </p>
          <!-- CTA -->
          <div style="text-align:center;margin:32px 0;">
            <a href="{{.DashboardURL}}/dashboard/applications"
               style="display:inline-block;background:#4f46e5;color:#ffffff;text-decoration:none;
                      padding:14px 32px;border-radius:8px;font-weight:600;font-size:15px;">
              View Dashboard
            </a>
          </div>
        </td></tr>
        <!-- Footer -->
        <tr><td style="background:#f9fafb;padding:24px 40px;border-top:1px solid #e5e7eb;">
          <p style="margin:0;color:#9ca3af;font-size:12px;text-align:center;">
            © {{.Year}} AutoDreamApplier &nbsp;·&nbsp;
            <a href="{{.DashboardURL}}/settings" style="color:#9ca3af;">Manage notifications</a>
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`

// ─── Failed ───────────────────────────────────────────────────────────────────

const htmlFailed = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Application Failed</title>
</head>
<body style="margin:0;padding:0;background:#f4f6f9;font-family:Inter,Helvetica,Arial,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="background:#f4f6f9;padding:40px 0;">
    <tr><td align="center">
      <table width="600" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 2px 8px rgba(0,0,0,.08);">
        <!-- Header -->
        <tr><td style="background:#4f46e5;padding:32px 40px;">
          <h1 style="margin:0;color:#ffffff;font-size:22px;font-weight:700;">AutoDreamApplier</h1>
        </td></tr>
        <!-- Body -->
        <tr><td style="padding:40px;">
          <p style="margin:0 0 8px;color:#374151;font-size:16px;">Hi {{.UserName}},</p>
          <h2 style="margin:16px 0;color:#111827;font-size:24px;">⚠️ Application attempt failed</h2>
          <p style="color:#6b7280;font-size:15px;line-height:1.6;">
            Unfortunately we couldn't complete your application to <strong>{{.JobTitle}}</strong>
            at <strong>{{.Company}}</strong>. Our team is monitoring for recurring issues.
          </p>
          <!-- Error card -->
          <table width="100%" style="background:#fef2f2;border:1px solid #fecaca;border-radius:8px;margin:24px 0;">
            <tr><td style="padding:20px;">
              <p style="margin:0 0 8px;font-size:13px;color:#dc2626;font-weight:600;text-transform:uppercase;letter-spacing:.05em;">
                What went wrong
              </p>
              <p style="margin:0;font-size:14px;color:#7f1d1d;line-height:1.5;">{{.ErrorSummary}}</p>
            </td></tr>
          </table>
          <p style="color:#6b7280;font-size:14px;line-height:1.6;">
            You can retry manually from the job listing page, or check the application
            history in your dashboard for details.
          </p>
          <!-- CTA -->
          <div style="text-align:center;margin:32px 0;">
            <a href="{{.DashboardURL}}/dashboard/applications"
               style="display:inline-block;background:#4f46e5;color:#ffffff;text-decoration:none;
                      padding:14px 32px;border-radius:8px;font-weight:600;font-size:15px;">
              View Application History
            </a>
          </div>
        </td></tr>
        <!-- Footer -->
        <tr><td style="background:#f9fafb;padding:24px 40px;border-top:1px solid #e5e7eb;">
          <p style="margin:0;color:#9ca3af;font-size:12px;text-align:center;">
            © {{.Year}} AutoDreamApplier &nbsp;·&nbsp;
            <a href="{{.DashboardURL}}/settings" style="color:#9ca3af;">Manage notifications</a>
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`

// ─── Outcome (interview / offer / rejected) ───────────────────────────────────

const htmlOutcome = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Application Update</title>
</head>
<body style="margin:0;padding:0;background:#f4f6f9;font-family:Inter,Helvetica,Arial,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="background:#f4f6f9;padding:40px 0;">
    <tr><td align="center">
      <table width="600" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 2px 8px rgba(0,0,0,.08);">
        <!-- Header -->
        <tr><td style="background:#4f46e5;padding:32px 40px;">
          <h1 style="margin:0;color:#ffffff;font-size:22px;font-weight:700;">AutoDreamApplier</h1>
        </td></tr>
        <!-- Body -->
        <tr><td style="padding:40px;">
          <p style="margin:0 0 8px;color:#374151;font-size:16px;">Hi {{.UserName}},</p>
          <h2 style="margin:16px 0;color:#111827;font-size:28px;">{{.OutcomeEmoji}} {{.Outcome}}!</h2>
          <p style="color:#6b7280;font-size:15px;line-height:1.6;">
            Great news! Your application to <strong>{{.JobTitle}}</strong> at
            <strong>{{.Company}}</strong> has been updated.
          </p>
          <!-- Outcome card -->
          <table width="100%" style="background:#f0fdf4;border:1px solid #bbf7d0;border-radius:8px;margin:24px 0;">
            <tr><td style="padding:24px;text-align:center;">
              <p style="margin:0;font-size:48px;">{{.OutcomeEmoji}}</p>
              <p style="margin:8px 0 0;font-size:20px;font-weight:700;color:#14532d;">{{.Outcome}}</p>
              <p style="margin:4px 0 0;font-size:14px;color:#15803d;">{{.JobTitle}} · {{.Company}}</p>
            </td></tr>
          </table>
          <p style="color:#6b7280;font-size:14px;line-height:1.6;">
            Log in to your dashboard to view full details and take your next steps.
          </p>
          <!-- CTA -->
          <div style="text-align:center;margin:32px 0;">
            <a href="{{.DashboardURL}}/dashboard/applications"
               style="display:inline-block;background:#4f46e5;color:#ffffff;text-decoration:none;
                      padding:14px 32px;border-radius:8px;font-weight:600;font-size:15px;">
              View Details
            </a>
          </div>
        </td></tr>
        <!-- Footer -->
        <tr><td style="background:#f9fafb;padding:24px 40px;border-top:1px solid #e5e7eb;">
          <p style="margin:0;color:#9ca3af;font-size:12px;text-align:center;">
            © {{.Year}} AutoDreamApplier &nbsp;·&nbsp;
            <a href="{{.DashboardURL}}/settings" style="color:#9ca3af;">Manage notifications</a>
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`

// ─── Weekly digest ────────────────────────────────────────────────────────────

const htmlWeeklyDigest = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Weekly Digest</title>
</head>
<body style="margin:0;padding:0;background:#f4f6f9;font-family:Inter,Helvetica,Arial,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="background:#f4f6f9;padding:40px 0;">
    <tr><td align="center">
      <table width="600" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 2px 8px rgba(0,0,0,.08);">
        <!-- Header -->
        <tr><td style="background:#4f46e5;padding:32px 40px;">
          <h1 style="margin:0;color:#ffffff;font-size:22px;font-weight:700;">AutoDreamApplier</h1>
          <p style="margin:4px 0 0;color:#c7d2fe;font-size:14px;">Weekly Activity Digest · {{.WeekOf}}</p>
        </td></tr>
        <!-- Body -->
        <tr><td style="padding:40px;">
          <p style="margin:0 0 8px;color:#374151;font-size:16px;">Hi {{.UserName}},</p>
          <h2 style="margin:16px 0 8px;color:#111827;font-size:22px;">📊 Here's your week at a glance</h2>
          <p style="margin:0 0 32px;color:#6b7280;font-size:14px;">
            AutoDreamApplier has been working hard on your behalf. Here's what happened:
          </p>
          <!-- Stats grid -->
          <table width="100%" cellpadding="0" cellspacing="0" style="margin-bottom:32px;">
            <tr>
              <td width="50%" style="padding:0 8px 16px 0;">
                <table width="100%" style="background:#eff6ff;border-radius:8px;">
                  <tr><td style="padding:20px;text-align:center;">
                    <p style="margin:0;font-size:36px;font-weight:700;color:#1d4ed8;">{{.Applied}}</p>
                    <p style="margin:4px 0 0;font-size:13px;color:#3b82f6;font-weight:500;">Applications Sent</p>
                  </td></tr>
                </table>
              </td>
              <td width="50%" style="padding:0 0 16px 8px;">
                <table width="100%" style="background:#f0fdf4;border-radius:8px;">
                  <tr><td style="padding:20px;text-align:center;">
                    <p style="margin:0;font-size:36px;font-weight:700;color:#16a34a;">{{.Interviews}}</p>
                    <p style="margin:4px 0 0;font-size:13px;color:#22c55e;font-weight:500;">Interview Requests</p>
                  </td></tr>
                </table>
              </td>
            </tr>
            <tr>
              <td width="50%" style="padding:0 8px 0 0;">
                <table width="100%" style="background:#fefce8;border-radius:8px;">
                  <tr><td style="padding:20px;text-align:center;">
                    <p style="margin:0;font-size:36px;font-weight:700;color:#ca8a04;">{{.Offers}}</p>
                    <p style="margin:4px 0 0;font-size:13px;color:#eab308;font-weight:500;">Offers Received</p>
                  </td></tr>
                </table>
              </td>
              <td width="50%" style="padding:0 0 0 8px;">
                <table width="100%" style="background:#f9fafb;border-radius:8px;border:1px solid #e5e7eb;">
                  <tr><td style="padding:20px;text-align:center;">
                    <p style="margin:0;font-size:36px;font-weight:700;color:#6b7280;">{{.Pending}}</p>
                    <p style="margin:4px 0 0;font-size:13px;color:#9ca3af;font-weight:500;">Awaiting Response</p>
                  </td></tr>
                </table>
              </td>
            </tr>
          </table>
          <!-- CTA -->
          <div style="text-align:center;margin:0 0 8px;">
            <a href="{{.DashboardURL}}/dashboard"
               style="display:inline-block;background:#4f46e5;color:#ffffff;text-decoration:none;
                      padding:14px 32px;border-radius:8px;font-weight:600;font-size:15px;">
              Open Dashboard
            </a>
          </div>
          <p style="text-align:center;color:#9ca3af;font-size:13px;">
            Keep going — consistency wins! 💪
          </p>
        </td></tr>
        <!-- Footer -->
        <tr><td style="background:#f9fafb;padding:24px 40px;border-top:1px solid #e5e7eb;">
          <p style="margin:0;color:#9ca3af;font-size:12px;text-align:center;">
            © {{.Year}} AutoDreamApplier &nbsp;·&nbsp;
            <a href="{{.DashboardURL}}/settings" style="color:#9ca3af;">Manage notifications</a>
            &nbsp;·&nbsp;
            <a href="{{.DashboardURL}}/unsubscribe" style="color:#9ca3af;">Unsubscribe</a>
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`
