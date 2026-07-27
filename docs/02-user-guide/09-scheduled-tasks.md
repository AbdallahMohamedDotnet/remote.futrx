# Scheduled tasks

Scheduled tasks let a project chat return to work later without keeping your
browser open. A task stores a prompt, a time or recurrence, an owner, and the
chat where every run will appear.

Use them for work such as:

- check a deployment once at a specific time;
- monitor a build every ten minutes until it finishes;
- produce a daily project report; or
- resume a bounded workflow later.

Schedules are available only in project chats. Loose chats do not show the
**Schedules** control and cannot own scheduled work.

## Create a schedule

The current UI manages existing schedules; the agent creates the initial
definition.

1. Open the project chat where the future runs should appear.
2. In **Skill set**, select **Scheduled Tasks**.
3. Ask the agent explicitly to schedule the work.
4. Include the intended time, timezone, recurrence, stopping condition, and
   maximum number of runs when those details matter.
5. Wait for the agent to report the task name, timing, timezone, and ID.
6. Select **Schedules** in the chat header.
7. Review the parked task and select **Arm**.

**Outcome:** Remote stores the task on the host. Agent-created tasks always
start paused, so nothing runs until a person reviews and arms them.

Example:

```text
Use Scheduled Tasks to check deployment dep_123 every 10 minutes in
America/Toronto. Report its current state each time. Stop when it is Ready or
Failed, and stop after 12 runs even if it never reaches a terminal state.
```

Do not use vague standing prompts such as “continue” or “check it.” A future
turn should be able to identify the exact project resource, action, completion
condition, and safety boundary from the saved prompt.

## Choose one-time or recurring execution

| Kind | Use it for | Time format |
| --- | --- | --- |
| One time | A reminder, deferred review, or future handoff | An exact date and time with an explicit timezone |
| Recurring | Monitoring, reports, or repeated maintenance | Standard five-field cron plus an IANA timezone |

Recurring expressions use:

```text
minute hour day-of-month month day-of-week
```

There is no seconds field. Use a timezone such as `UTC`,
`America/Toronto`, or `Europe/London`; do not rely on the container's local
timezone.

The default minimum interval is five minutes. The server rejects definitions
that would run more frequently. Operators can change that guardrail.

## Read the Schedules drawer

Select **Schedules** in a project chat to see the tasks visible to you for that
chat.

Each card shows:

- task name, status, and saved prompt;
- one-time date or cron expression and timezone;
- next run;
- last run time and result;
- owner;
- run count and optional maximum; and
- the last error, when one exists.

Enabled tasks appear first, ordered by next run time. Paused and terminal tasks
follow.

Use **Refresh scheduled tasks** when another browser, agent turn, or scheduled
run may have changed the state.

## Manage a task

| Control | Result |
| --- | --- |
| **Arm** | Enables a reviewed agent-created task for the first time |
| **Pause** | Stops future automatic fires while preserving the definition and history |
| **Resume** | Re-enables a paused task |
| **Run now** | Requests one immediate run without moving the regular deadline |
| **Edit** | Changes name, prompt, time or cron, timezone, and maximum runs |
| **Delete** | Removes the definition and its recorded run history after confirmation |

Use **Pause** when the task may be needed again. Use **Delete** only when the
definition and its history are no longer useful.

Completed, exhausted, and error tasks are terminal and cannot be resumed from
the drawer. Create a new task when that standing job should start again.

## What happens when a task fires

A scheduled fire is a normal turn in the same chat:

1. The backend claims and records the occurrence.
2. It starts the project container if needed.
3. It submits the saved prompt through the same one-run-per-chat service as a
   prompt typed by a user.
4. The user prompt, streamed output, tools, errors, and completion appear in
   the chat transcript.
5. The task records the result and calculates its next deadline.

The selected provider session is resumed, so repeated runs can accumulate
provider context and token usage. Use a sensible `maxRuns` for monitoring and
other bounded work.

If the chat is already busy, Remote does not stack simultaneous runs. The
default overlap policy coalesces missed occurrences into one follow-up when the
chat becomes available. A long outage or busy period therefore produces at
most one catch-up run, not a replay of every missed interval.

**Run now** also respects the chat's single-run boundary. If the chat is busy,
the requested run becomes the one pending follow-up. It does not change the
task's normal one-time or cron deadline.

## Finish a standing task

A monitoring task can stop itself after its actual goal becomes complete. The
scheduled-task skill tells the agent how to mark the current task complete.

For example, a deployment watcher should complete itself only after the
deployment is in its declared terminal state—not merely because one check ran
successfully. If completion is ambiguous, the agent should report the evidence
and leave the task active.

When the task reaches `maxRuns`, Remote marks it exhausted and stops future
fires.

## Ownership and access

- A schedule belongs to the user who created it or asked the agent to create
  it.
- A project member sees and manages their own schedules.
- An administrator can see and manage every schedule.
- Every fire re-checks that the owner is still registered and still has access
  to the project.
- If the chat is deleted, the project relationship changes, the owner is
  removed, or access is revoked, Remote pauses the task in an error state
  instead of continuing unattended.

Removing a person from a project does not transfer ownership of their tasks.
Create replacement tasks under the intended owner when responsibility changes.

## Server guardrails

The default installation enforces:

| Guardrail | Default |
| --- | ---: |
| Minimum recurrence interval | 5 minutes |
| Simultaneous scheduled runs across the server | 2 |
| Standing tasks per project | 20 |

Terminal tasks do not consume the per-project standing-task quota. Operators
can change or disable these limits through service environment variables; see
[Deployment and operations](../04-operations/09-deployment-and-operations.md#scheduled-task-guardrails).

For the scheduler, capability-token, overlap, persistence, and crash-recovery
design, read [Scheduled tasks architecture](../02-workspaces/06-scheduled-tasks.md).
