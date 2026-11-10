# API contract

All routes are under `/api/v1`. JSON keys are camelCase. Timestamps are RFC 3339 UTC. Dates are `YYYY-MM-DD`. Mutating routes require `Content-Type: application/json` unless stated, with bodies up to 1 MB and unknown fields rejected.

The browser only talks to the Next.js app, which forwards `/api/v1/*` to the API at runtime using `API_URL`. Server Components call the API directly and forward the incoming cookie. The API has no CORS.

## Conventions

**Errors:** `{"error":{"code":"snake_case","message":"Sentence to show the user"}}`. Status 400 validation, 401 not signed in, 403 forbidden, 404 missing or not visible, 409 conflict or wrong state, 413 too large, 503 no database. Validation errors use code `validation_failed` and may add `"fields":{"name":"Message"}`. A wrong content type returns 400 `unsupported_content_type`, and this includes POST and DELETE routes with no body: send `Content-Type: application/json` on them too.

**Common codes:** 404 `project_not_found`, `role_not_found`, `request_not_found`, `student_not_found`. 403 `not_lead` on lead-only routes, for teams anyone can see.

**Pages:** `?page=1&pageSize=12` (max 50). Response is `{"items":[...],"page":1,"pageSize":12,"total":34}`.

**Auth:** every route needs a session cookie (`cu_session`) except `GET /health` and `/auth/*`.

**Lazy time rules:** there are no timers, because free hosting sleeps. Due requests are expired when requests or notifications are read and before a request is created or decided, and notify both sides. End-date reminders are written the same way. A past-due request reads as `expired` everywhere, even before that sweep reaches it.

## Health

`GET /api/v1/health` is a public process-liveness check. It returns HTTP 200, `Content-Type: application/json`, and `{"status":"ok"}`. It does not check database readiness or require credentials. HEAD is supported with an empty body; other methods return 405. Unknown paths return 404.

The API listens on `0.0.0.0:$PORT`, defaulting to port 3001. An invalid or occupied port fails startup. SIGINT and SIGTERM allow up to ten seconds for active requests to finish. With `DATABASE_URL` set, migrations run at startup. Without it the API still starts, health works, and other routes return 503.

## Shared shapes

```ts
UserRef      { id, displayName, faculty: string|null, year: number|null }   // closed accounts: displayName "Former student"
Rating       { average: number|null, count: number }                        // published scores only; null average means "No ratings yet"
ProfileCard  { userId, displayName, faculty, year, headline, skills: string[] /* up to 5 */, rating: Rating }
Role         { id, name, capacity, taken, available }
Member       { userId, displayName, faculty, roleId, roleName, joinedAt, isLead }
ProjectCard  { id, name, description, category: {id,name}, status, recruitmentStatus,
               durationValue, durationUnit, placesTaken, placesTotal,
               lead: {id, displayName, rating: Rating}, roles: Role[] }
Request      { id, type: "application"|"invitation", status, cancelReason, message,
               project: {id, name, status, category: {id,name}}, role: {id, name},
               student: UserRef & {rating: Rating}, invitedBy: {id, displayName}|null,
               createdAt, expiresAt, resolvedAt,
               viewer: { isStudent, isLead, canApprove, canReject, canWithdraw, canAccept, canDecline, canCancel } }
```

`status` values: `not_started`, `in_progress`, `completed`, `cancelled`. Request statuses and cancel reasons are the enums in the [schema doc](database-schema.md).

## Accounts

| Route                     | Body      | Result                                                                                                                                                                                                                                          |
| ------------------------- | --------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /auth/demo-sign-in` | `{email}` | Only when `DEMO_SIGN_IN=true`, otherwise 404. Email must end in `@student.chula.ac.th`, or 400. 403 `account_closed` for a closed account. Sets cookie, returns `Me`                                                                            |
| `GET /auth/demo-accounts` |           | Only when `DEMO_SIGN_IN=true`. `{items:[{email, displayName, note}]}`: seeded accounts for the sign-in page                                                                                                                                     |
| `POST /auth/sign-out`     |           | 204. Revokes the session, clears the cookie                                                                                                                                                                                                     |
| `GET /me`                 |           | `Me { id, email, displayName, profileComplete, unreadNotifications }`, or 401                                                                                                                                                                   |
| `POST /me/close`          | `{}`      | 204. 409 `must_transfer_or_delete` while leading a team that hasn't ended. Cancels pending requests (`account_closed`), ends memberships on teams that haven't ended, revokes sessions, sets `closed_at`. Profile and ratings disappear at once |
| `GET /faculties`          |           | `{items:[{id,name}]}`                                                                                                                                                                                                                           |
| `GET /categories`         |           | `{items:[{id,name}]}`                                                                                                                                                                                                                           |
| `GET /skills?q=`          |           | `{items:[{id,name,isPredefined}]}`, up to 20, partial case-insensitive; predefined first                                                                                                                                                        |

Only the signed-in user's own email is ever returned.

## Profiles

| Route                           | Body                                                                       | Result                                                                                                                                                                                                                                                 |
| ------------------------------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `GET /me/profile`               |                                                                            | `MyProfile`: `Profile` plus `email`, `cvShared`, `searchable`                                                                                                                                                                                          |
| `PUT /me/profile`               | `{displayName, facultyId, year, headline, workingStyle, skills: string[]}` | Upsert. All nullable or empty so a partial profile can save. Skill names are matched on `normalized_name`; unknown ones are created as custom. Returns `MyProfile`                                                                                     |
| `PUT /me/availability`          | `{slots:[{day:1-7, start:"HH:MM"}]}`                                       | Replaces all slots and returns `{slots}`. `start` must be on `:00` or `:30`, Bangkok time                                                                                                                                                              |
| `POST /me/cv`                   | multipart, field `file`                                                    | Upload or replace. 413 over 5 MB. 400 `not_pdf` unless a PDF parser structurally validates its cross-references, catalog and page tree within parsing limits and without a password. Returns `{fileName, sizeBytes, uploadedAt}`                       |
| `DELETE /me/cv`                 |                                                                            | 204                                                                                                                                                                                                                                                    |
| `PUT /me/cv/sharing`            | `{shared: boolean}`                                                        | `{shared}`                                                                                                                                                                                                                                             |
| `GET /users/{id}/cv`            |                                                                            | The PDF, `Content-Disposition: inline`. Owner always. Others only while `cv_shared` is true, checked on every request. Else 403 `cv_not_shared`, or 404 with no CV                                                                                     |
| `GET /profiles?q=&skill=&page=` |                                                                            | Page of `ProfileCard`, searchable profiles only, including the viewer. `q` matches display name or skills by partial case-insensitive text. Legacy `skill` matches skills only when `q` is blank. Newest first; matching skills come first in `skills` |
| `GET /profiles/{id}`            |                                                                            | `Profile`. 404 when closed or without a display name                                                                                                                                                                                                   |
| `GET /profiles/{id}/history`    |                                                                            | `{current: HistoryItem[], past: HistoryItem[]}`                                                                                                                                                                                                        |
| `GET /profiles/{id}/ratings`    |                                                                            | `{rating: Rating, published: [{score, reviewer:{id,displayName}, project:{id,name}}], pendingProjects: [{id,name}]}`                                                                                                                                   |
| `GET /profiles/{id}/teammates`  |                                                                            | `{items:[{teammate:{id,displayName}, project:{id,name}, state:"published"\|"pending"\|"not_rated", score:number\|null}]}`: what each past teammate gave this student                                                                                   |

The People search uses `q`. A nonblank `q` takes precedence over `skill`; `%` and `_` are literal characters. Names do not bypass profile visibility rules, and email addresses are neither searched nor returned.

```ts
Profile {
  userId, displayName, faculty: {id,name}|null, year, headline, workingStyle: string|null,
  skills: string[], availability: [{day, start}], rating: Rating, isSelf,
  cv: { available: boolean, fileName: string|null },   // available only if viewer may open it
  viewerActions: { invitableProjects: [{id, name, roles: [{id, name, available}]}] }  // teams the viewer leads with open recruitment
}
HistoryItem { project: {id,name,status}, roleName, joinedAt, leftAt, leftReason, endedAt }
```

A profile is searchable with a display name, a faculty, at least one skill, and an open account. `profileComplete` in `Me` means the same. Posting, applying and inviting need a complete profile, else 409 `profile_incomplete`.

The past-teammate view includes overlapping participation on completed or cancelled teams and memberships that ended. Current pairs still together on an active team are omitted. It returns one row per teammate and project. Cancelled projects and former members show `not_rated`; only eligible completed pairs can be `pending` or have a published score. Closed accounts are omitted.

History: current holds active memberships on `not_started` and `in_progress` teams. Past holds `completed` and `cancelled` teams, plus any membership that ended. Deleted teams are left out. `endedAt` is `left_at`, or the team's `completed_at` or `cancelled_at`.

## Projects

| Route                                    | Body                                                                                                    | Result                                                                                                                                                                                                                                      |
| ---------------------------------------- | ------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /projects?q=&category=&page=`       |                                                                                                         | Page of `ProjectCard`: open recruitment and at least one free place only, never deleted. `q` is partial case-insensitive on name, description and role names. `category` is a category id                                                   |
| `POST /projects`                         | `{name, description, categoryId, durationValue, durationUnit, roles:[{name, capacity}], leadRoleIndex}` | 201 `ProjectDetail`. Creator becomes lead and first member in `roles[leadRoleIndex]`. At least one place must remain free after that                                                                                                        |
| `GET /me/projects`                       |                                                                                                         | `{items: MyProject[]}`: `ProjectCard` plus `{isLead, roleName, ratingsOpen, hasPendingTransferToMe}`, current memberships, including ended teams                                                                                            |
| `GET /projects/{id}`                     |                                                                                                         | `ProjectDetail`. 404 when deleted                                                                                                                                                                                                           |
| `GET /projects/{id}/similar`             |                                                                                                         | `{items: ProjectCard[]}`, up to 5 joinable teams in the same category                                                                                                                                                                       |
| `PATCH /projects/{id}`                   | any of `{name, description, categoryId, durationValue, durationUnit}`                                   | Lead. Returns `ProjectDetail`. 409 `team_ended` on ended teams                                                                                                                                                                              |
| `POST /projects/{id}/roles`              | `{name, capacity}`                                                                                      | Lead. Returns the `Role`. 409 `role_name_taken`, 409 `team_ended`                                                                                                                                                                           |
| `PATCH /projects/{id}/roles/{roleId}`    | any of `{name, capacity}`                                                                               | Lead. Returns the `Role`. 409 `capacity_below_members` below the current member count, 409 `role_name_taken`, 409 `team_ended`. Lowering to exactly the member count fills the role, so its pending requests are cancelled with `role_full` |
| `DELETE /projects/{id}/roles/{roleId}`   |                                                                                                         | Lead. 204. 409 `role_has_members`, 409 `team_ended`. Pending requests become `cancelled` with `role_removed`, students notified                                                                                                             |
| `POST /projects/{id}/recruitment`        | `{status:"open"\|"closed"}`                                                                             | Lead. Returns `ProjectDetail`. Closing cancels pending requests with `recruitment_closed` and notifies. Reopening restores nothing. 409 `team_ended` on ended teams                                                                         |
| `PUT /projects/{id}/contact`             | `{lineUrl, discordUrl}`                                                                                 | 200 `ProjectDetail`. Lead. Null clears. HTTPS only. `line.me` or `*.line.me` for LINE; `discord.gg`, `discord.com/invite` for Discord. Credentials, lookalike hosts, and ports other than 443 are rejected. 400 `unsupported_link`          |
| `POST /projects/{id}/start`              | `{expectedEndDate}`                                                                                     | 200 `ProjectDetail`. Lead, `not_started` only, else 409 `project_already_started`. 409 `recruitment_open`. Date must be today or later, Bangkok                                                                                             |
| `POST /projects/{id}/extend`             | `{expectedEndDate}`                                                                                     | 200 `ProjectDetail`. Lead, `in_progress` only, else 409 `project_not_in_progress`. Date must be later than the current one. Clears the reminder                                                                                             |
| `POST /projects/{id}/complete`           | `{}`                                                                                                    | 200 `ProjectDetail`. Lead, `in_progress` only. Records `completed_at`, closes recruitment, cancels pending with `project_completed`, prompts every current member to rate                                                                   |
| `POST /projects/{id}/cancel`             | `{}`                                                                                                    | 200 `ProjectDetail`. Lead, `in_progress` only. Closes recruitment, cancels pending with `project_cancelled`, notifies. No ratings                                                                                                           |
| `DELETE /projects/{id}`                  |                                                                                                         | 204. Lead, `not_started` only, else 409 `project_started`. Sets `deleted_at`, ends memberships with `team_deleted`, cancels pending with `team_deleted`, notifies                                                                           |
| `POST /projects/{id}/leave`              | `{}`                                                                                                    | 204. Non-lead member. 409 `lead_must_transfer` for the lead, 409 `team_ended` on ended teams. Notifies the lead                                                                                                                             |
| `DELETE /projects/{id}/members/{userId}` |                                                                                                         | 204. Lead, not self. 409 `team_ended`. Notifies the removed member                                                                                                                                                                          |

```ts
ProjectDetail = ProjectCard & {
  createdAt, startedAt, completedAt, cancelledAt, expectedEndDate,
  members: Member[],              // current members only
  contact: { lineUrl, discordUrl } | null,   // null unless the viewer is a current member
  pendingTransfer: { id, toUserId, toDisplayName, fromUserId } | null,  // lead and recipient only
  ratingWindow: { closesAt, open: boolean } | null,                     // completed teams only
  endDateReached: boolean,        // lead banner: complete or extend. Always false for anyone else
  viewer: {
    isLead, isMember,
    pendingRequest: Request | null,
    applyBlock: { code, message } | null   // team-wide reason this viewer can't apply
  }
}
```

`applyBlock.code`: `lead_or_member`, `pending_request`, `previously_rejected`, `recruitment_closed`, `team_ended`, `profile_incomplete`. A full role shows `available: 0`, and the UI explains it. A role is full when `taken >= capacity`. Removed roles never appear and never count toward `placesTaken` or `placesTotal`.

Lifecycle dates use Bangkok's current date. Invalid or missing dates return 400 `validation_failed`. Complete and cancel require `in_progress`, otherwise 409 `project_not_in_progress`; terminal teams return 409 `team_ended` on lifecycle, membership, and transfer changes. Contact links can still be replaced by the lead after a team ends. Removing yourself through the member route returns 403 `not_allowed`; use transfer and leave instead. A missing current member returns 404 `member_not_found`.

## Leadership transfers

| Route                           | Body         | Result                                                                                                                    |
| ------------------------------- | ------------ | ------------------------------------------------------------------------------------------------------------------------- |
| `POST /projects/{id}/transfers` | `{toUserId}` | 201 `Transfer`. Lead. Recipient must be another current member. 409 `transfer_pending` when one is pending. Notifies them |
| `POST /transfers/{id}/accept`   | `{}`         | 200 `Transfer`. Recipient. Recipient becomes lead, the old lead stays a member and can now leave. Notifies both           |
| `POST /transfers/{id}/decline`  | `{}`         | 200 `Transfer`. Recipient. Notifies the lead                                                                              |
| `POST /transfers/{id}/cancel`   | `{}`         | 200 `Transfer`. Lead. Notifies the recipient                                                                              |

```ts
Transfer { id, projectId, fromUserId, toUserId, status: "pending"|"accepted"|"declined"|"cancelled" }
```

A pending transfer is cancelled if the recipient leaves or is removed, either account closes, or the project completes, is cancelled, or is deleted. Membership and leadership are frozen on completed and cancelled teams. A second decision returns 409 `transfer_not_pending`. An unrelated viewer receives 404 `transfer_not_found`; a participant using the wrong action receives 403 `not_allowed`. Self-transfer returns 403 `not_allowed`; a missing or closed current recipient returns 404 `member_not_found`.

## Requests

| Route                              | Body                  | Result                                                                                                                                                          |
| ---------------------------------- | --------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /projects/{id}/applications` | `{roleId, message?}`  | 201 `Request`. Notifies the lead                                                                                                                                |
| `POST /projects/{id}/invitations`  | `{roleId, studentId}` | Lead. 201 `Request`. Notifies the student                                                                                                                       |
| `GET /projects/{id}/requests`      |                       | Lead. `{items: Request[]}`, newest first                                                                                                                        |
| `GET /me/requests`                 |                       | `{sent: Request[], received: Request[]}`. Sent: my applications and invitations from teams I lead. Received: invitations to me and applications to teams I lead |
| `GET /requests/{id}`               |                       | `Request`. Student and current lead only, else 404                                                                                                              |
| `POST /requests/{id}/approve`      | `{}`                  | Lead, application. Fills the place                                                                                                                              |
| `POST /requests/{id}/reject`       | `{}`                  | Lead, application                                                                                                                                               |
| `POST /requests/{id}/withdraw`     | `{}`                  | Student, application                                                                                                                                            |
| `POST /requests/{id}/accept`       | `{}`                  | Student, invitation. Fills the place                                                                                                                            |
| `POST /requests/{id}/decline`      | `{}`                  | Student, invitation                                                                                                                                             |
| `POST /requests/{id}/cancel`       | `{}`                  | Lead, invitation                                                                                                                                                |

All of them notify the other side and return the updated `Request`. Creating a request returns 409 with one of: `already_member`, `pending_request`, `previously_rejected` (the lead must invite), `previously_declined` (the student must apply), `recruitment_closed`, `role_full`, `team_ended`, `profile_incomplete`. A student can't apply to a team they lead or belong to, and a lead can't invite themselves or a current member (`already_member`). Inviting a closed account or a student with no display name returns 404 `student_not_found`.

Deciding returns 409 `request_not_pending`, `request_expired`, or `role_full`. Approving and accepting can also return 409 `recruitment_closed`, `team_ended`, or `already_member`. An action that doesn't fit the request returns 409 `wrong_request_type`, and the wrong side of the request gets 403 `not_allowed`. Anyone who isn't the student or the current lead gets 404. Any decision on a past-due request returns 409 `request_expired` and marks it expired. Filling the last place cancels other pending requests for that role with `role_full` and notifies those students. Joining one team never cancels requests to other teams.

The retry rule reads the latest request for that student and team. See the [schema doc](database-schema.md#rules-the-app-must-enforce).

## Ratings

| Route                                      | Body      | Result                                                                                                                                                                                                                              |
| ------------------------------------------ | --------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /projects/{id}/ratings`               |           | `ProjectRatings`. 409 `project_not_completed` before completion                                                                                                                                                                     |
| `PUT /projects/{id}/ratings/{recipientId}` | `{score}` | 200 `{recipientId, score}`. Upsert. Reviewer and recipient must be current members at completion and different people, else 403. 400 `score_required` for a missing or out-of-range score. 409 `rating_window_closed` after 14 days |

```ts
ProjectRatings {
  closesAt, open: boolean, eligible: boolean,
  teammates: [{ user: {id, displayName}, roleName, myScore: number|null }],  // eligible viewers only
  published: [{ reviewer: {id,displayName}, recipient: {id,displayName}, score }],  // empty until the window closes
  statusText: "Ratings pending" | null   // shown to viewers during the window
}
```

Ineligible pairs, including self-ratings, return 403 `not_allowed`. A missing rating is never zero. Ratings from or about closed accounts are left out of averages and lists.

## Notifications

| Route                              | Result                                                                                                                                        |
| ---------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /notifications?unread=&page=` | Page of `{id, type, message, link, createdAt, readAt}` plus `unreadCount`. The API writes `message` and `link`, so the web only displays them |
| `POST /notifications/{id}/read`    | 204                                                                                                                                           |
| `POST /notifications/read-all`     | 204                                                                                                                                           |

Types: `application_received`, `application_approved`, `application_rejected`, `application_withdrawn`, `invitation_received`, `invitation_accepted`, `invitation_declined`, `invitation_cancelled`, `request_expired`, `request_cancelled`, `member_left`, `member_removed`, `transfer_requested`, `transfer_accepted`, `transfer_declined`, `transfer_cancelled`, `team_deleted`, `project_started`, `project_completed`, `project_cancelled`, `end_date_reached`, `rate_teammates`.

Project-only notifications for a deleted team link to `/notifications`; request notifications still link to the readable request.

`link` is a web path: `/projects/{id}`, `/projects/{id}/rate`, `/requests/{id}`, or `/notifications` when there is nothing to open, such as `team_deleted`.
