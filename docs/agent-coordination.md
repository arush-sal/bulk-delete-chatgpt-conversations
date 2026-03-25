# Agent Coordination Guide

Use this guide when multiple agents are working on the same repo or delivery stream.

## Coordination Rules

1. Give each worker a stable name early.
   Use short, durable names like `engineer-1`, `reviewer-1`, or `qa-1` so instructions and follow-ups stay unambiguous.

2. Keep one task and one PR per agent.
   Do not have one agent juggle unrelated workstreams or stack multiple unrelated fixes into the same PR.

3. Define the control mode up front.
   State whether the session is hands-off, approval-driven, or update-driven before work starts.

4. Define the update cadence explicitly.
   Say when updates are expected, such as every 30 minutes, on blockers only, or at each phase change.

5. Define the cleanup policy once.
   Decide in advance when branches, temporary environments, and inactive agents should be closed or removed.

6. Separate implementation, review, and QA roles.
   Do not collapse all responsibilities into one agent when the change is substantial enough to benefit from a second set of eyes.

7. Define merge authority clearly.
   Specify who can approve, who can merge, and whether merge requires reviewer or QA confirmation.

8. Define what counts as done.
   Include the expected code changes, validation steps, documentation updates, and PR state so agents stop at the same finish line.

9. Use concrete correction feedback.
   Say exactly what is wrong, where it is wrong, and what acceptable correction looks like instead of giving vague rework requests.

10. Ask for structured status updates.
    Require a consistent format such as `status`, `current step`, `blockers`, `next action`, and `PR link` so progress is easy to scan.

## Recommended Default Policy

- Use named agents only.
- Split engineer and reviewer roles for substantial PRs.
- Auto-clean merged agents and their temporary state.
- Notify only on blockers, PR opened, ready to merge, and merged.

## Example Status Template

```text
Status: in progress | blocked | ready for review | ready to merge | merged
Current step: <what the agent is doing now>
Blockers: <none or concrete blocker>
Next action: <next planned step>
PR: <link or not opened yet>
```
