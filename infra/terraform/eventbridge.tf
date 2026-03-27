# ── Job Discovery — every 2 hours ─────────────────────────────────────────

resource "aws_cloudwatch_event_rule" "job_discovery" {
  name                = "${var.project_name}-job-discovery"
  description         = "Trigger job discovery Lambda every 2 hours"
  schedule_expression = "rate(2 hours)"
}

resource "aws_cloudwatch_event_target" "job_discovery" {
  rule      = aws_cloudwatch_event_rule.job_discovery.name
  target_id = "JobDiscoveryLambda"
  arn       = aws_lambda_function.job_discovery.arn
}

resource "aws_lambda_permission" "job_discovery_eventbridge" {
  statement_id  = "AllowEventBridgeInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.job_discovery.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.job_discovery.arn
}

# ── Job Matcher — 30 minutes after discovery ──────────────────────────────

resource "aws_cloudwatch_event_rule" "job_matcher" {
  name                = "${var.project_name}-job-matcher"
  description         = "Trigger job matcher Lambda every 2 hours (offset 30 min)"
  schedule_expression = "cron(30 */2 * * ? *)"
}

resource "aws_cloudwatch_event_target" "job_matcher" {
  rule      = aws_cloudwatch_event_rule.job_matcher.name
  target_id = "JobMatcherLambda"
  arn       = aws_lambda_function.job_matcher.arn
}

resource "aws_lambda_permission" "job_matcher_eventbridge" {
  statement_id  = "AllowEventBridgeInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.job_matcher.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.job_matcher.arn
}

# ── Follow-Up Scheduler — every hour ──────────────────────────────────────

resource "aws_cloudwatch_event_rule" "followup_scheduler" {
  name                = "${var.project_name}-followup-scheduler"
  description         = "Trigger follow-up scheduler Lambda every hour"
  schedule_expression = "rate(1 hour)"
}

resource "aws_cloudwatch_event_target" "followup_scheduler" {
  rule      = aws_cloudwatch_event_rule.followup_scheduler.name
  target_id = "FollowupSchedulerLambda"
  arn       = aws_lambda_function.followup_scheduler.arn
}

resource "aws_lambda_permission" "followup_scheduler_eventbridge" {
  statement_id  = "AllowEventBridgeInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.followup_scheduler.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.followup_scheduler.arn
}
