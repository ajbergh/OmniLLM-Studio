# OmniLLM-Studio UI/UX Audit

**Audit date:** July 12, 2026  
**Target:** `http://localhost:5173/`  
**Viewports:** 1440 × 1000 desktop and 390 × 844 mobile  
**Method:** Playwright interaction, accessibility snapshots, screenshots, console review, DOM measurements, and a focused source review of the front-end implementation.

## Remediation status

**Overall:** Complete  
**Started:** July 12, 2026  
**Last updated:** July 12, 2026  
**Verified complete:** 32 / 32 tracked work items

Status legend: `Not started` · `In progress` · `Implemented` · `Verified` · `Blocked`

| ID | Audit area | Status | Progress / validation |
| --- | --- | --- | --- |
| UIUX-001 | Image Edit mobile toolbar and canvas | Verified | Edge-constrained scrolling toolbar, 44px mobile targets, and visibility-aware canvas fitting pass the 390px Image Studio regression. |
| UIUX-002 | Music Studio mobile workspace | Verified | Full-height Create/Result/History tabs with independent panels pass the mobile media regression. |
| UIUX-003 | Video Studio mobile workspace | Verified | Create/Preview/History/Outputs tabs and sticky generation actions pass the mobile media regression. |
| UIUX-004 | Video Edit mobile experience | Verified | Preview/Timeline/Media/Inspector workspaces and compact quick actions pass the mobile media regression. |
| UIUX-005 | Conversation switching state integrity | Verified | Loaded-conversation gating, immediate skeleton replacement, disabled actions, a unit test, and a delayed-network Playwright test all pass. |
| UIUX-006 | Branch transcript loading | Verified | Selected branch messages replace the rendered transcript; main/failure recovery is explicit and branch replacement has unit coverage. |
| UIUX-007 | Image provider capability resolution | Verified | Resolution uses backend flags, configured image defaults, and the shared catalog; setup offers a direct Settings action. Build passes. |
| UIUX-008 | Image History nested buttons | Verified | History selection and branch actions are sibling controls; the mobile Image flow passes without nested-control console errors. |
| UIUX-009 | Mobile drawer mode navigation | Verified | Every mode closes the drawer; exit animation no longer duplicates the trigger and studio data loads outside the drawer lifecycle. |
| UIUX-010 | Overlay stack and model selector | Verified | Global dialogs dismiss popovers; Escape/focus return and listbox semantics are implemented; dedicated Playwright coverage passes. |
| UIUX-011 | Search result context and ranking UI | Verified | Results show conversation/project, one role, date, highlighted context, and no raw score. Production build passes. |
| UIUX-012 | Workspace restoration and deep links | Verified | Chat, image, music, video, and edit URLs synchronize and restore their active record. Production build passes. |
| UIUX-013 | Guided empty states | Verified | Image, conversation, Plugin, and Eval empty states now provide a primary next action and explanatory copy. |
| UIUX-014 | Duplicate studio navigation | Verified | Global sidebar owns Music sessions and Video projects; duplicate create-pane lists were removed. Mobile navigation regression passes. |
| UIUX-015 | Music playback controls | Verified | One branded player now owns playback, seek/current time, loop, volume, and download. Build and mobile workspace tests pass. |
| UIUX-016 | Touch density and target sizing | Verified | Coarse-pointer density enforces 44px controls; mobile canvas/workspace/branch/Copy actions are touch-sized and overflow checks pass. |
| UIUX-017 | Keyboard/semantic list interaction | Verified | Sidebar rows, action menus, model options, arrow-key resizers, and Copy discovery are keyboard/focus accessible. Lint passes. |
| UIUX-018 | Modal and form consistency | Verified | Settings locks scroll, traps focus, handles Escape/return focus, labels shared fields, and exposes switch state; mobile dialog regression passes. |
| UIUX-019 | Live status announcements | Verified | Loading, thinking, uploads, media progress, and failures expose status/alert semantics. Slow-network and build tests pass. |
| UIUX-020 | Reduced motion | Verified | CSS honors `prefers-reduced-motion`; Framer Motion uses `reducedMotion="user"`. Production build passes. |
| UIUX-021 | Theme tokens and contrast | Verified | Runtime/build `surface-light` and `background` aliases exist for every theme; Cloud muted contrast was raised. Type/build checks pass. |
| UIUX-022 | Local-first font request | Verified | Google Fonts/preconnect requests were removed; the system stack is local and the built HTML contains no external font dependency. |
| UIUX-023 | Provider readiness onboarding | Verified | Dead-end chat creation is prevented with a direct provider Settings path; image/music/video setup explains configured capability requirements. |
| UIUX-024 | Loading/empty/error separation | Verified | Conversation lists distinguish skeleton, retryable error, and empty; studios expose loading/progress/error/empty states and App owns loading lifecycle. |
| UIUX-025 | Attachment size consistency | Verified | Validation and copy derive from one 50 MB constant. Build and lint pass. |
| UIUX-026 | Navigation/command palette/list scale | Verified | Durable routes, global Tools/Search access, keyboard shortcuts, and content-visibility list containment are implemented. |
| UIUX-027 | Commercial model picker | Verified | Search, configured availability, free/tool-capability badges, custom models, dynamic Ollama models, and recent-model ordering are implemented. |
| UIUX-028 | Evaluation and template workflows | Verified | Eval has a no-code multi-case builder plus Advanced JSON; Templates are searchable across name/category/description/content. Build passes. |
| UIUX-029 | Plugin governance | Verified | Plugins expose install/refresh, enable/disable, runtime health, declared capabilities/tools, permissions, local-unsigned trust, install date, and uninstall. |
| UIUX-030 | Usage and cost controls | Verified | Provider/model breakdown, periods, local budget threshold/alert, pricing rules, and CSV export are implemented. Mobile pricing regression passes. |
| UIUX-031 | Media workflow consistency | Verified | Studios share explicit mobile workspace navigation, progress/error states, primary actions, cross-studio handoffs, and persistent edit save status. |
| UIUX-032 | Automated regression coverage | Verified | 8/8 isolated Playwright regressions and 29/29 unit tests pass, including mobile overflow, overlays, slow switching, and branch isolation. |

### Progress log

- **July 12, 2026 — Remediation started:** Converted the audit into a living 32-item tracker. Began the release-blocker pass covering chat state integrity, branch loading, Image Edit, provider capability detection, invalid markup, and mobile drawer behavior.
- **July 12, 2026 — Baseline captured:** Frontend production build passes; 27/27 unit tests pass; ESLint passes with four pre-existing warnings (ChatView hook dependency plus three Fast Refresh export warnings). Vite reports a pre-existing 1.36 MB main chunk warning.
- **July 12, 2026 — Critical batch implemented:** Added conversation-ID loading isolation and composer gating; rendered selected branch transcripts; repaired image provider capability derivation; made the mobile canvas toolbar scrollable with touch-sized controls and automatic fit; removed Image History's nested button; and closed the mobile drawer after mode changes. Production build passes and ESLint now reports three pre-existing Fast Refresh warnings (the ChatView dependency warning was resolved).
- **July 12, 2026 — Mobile studios implemented:** Replaced stacked mobile desktop panes with Music Create/Result/History, Video Create/Preview/History/Outputs, and Video Edit Preview/Timeline/Media/Inspector workspaces. Added sticky/compact primary actions and removed duplicate Music/Video navigation. ESLint and 27/27 unit tests pass.
- **July 12, 2026 — Shared UX/accessibility batch implemented:** Added overlay dismissal and model-picker keyboard semantics; deep-link restoration; contextual search results; a single custom music player; keyboard resizers and sidebar rows; live status/alert regions; reduced-motion handling; complete surface aliases and improved Cloud contrast; system fonts with no Google request; provider dead-end prevention; distinct conversation loading/error/empty states; and consistent 50 MB attachment feedback. Production build passes; three pre-existing Fast Refresh warnings remain.
- **July 12, 2026 — Commercial workflow pass implemented:** Added recent-aware model ordering, a no-code Eval case builder, searchable Templates, guided Plugin/Eval empty states, plugin trust/permission/runtime metadata, Usage budgets and threshold alerts, CSV export, coarse-pointer density, and scalable list containment.
- **July 12, 2026 — Remediation verified complete:** Production build passes; ESLint has zero errors and three pre-existing Fast Refresh warnings; 29/29 unit tests pass; and 8/8 Chromium Playwright remediation tests pass. Playwright also uncovered and verified fixes for duplicate mobile drawer triggers and drawer-coupled studio loading.

## Executive summary

OmniLLM-Studio already has a strong visual foundation: the dark theme is cohesive, cross-studio workflows are unusually capable, the desktop Image Canvas is polished, and the mobile Tools and Settings overlays adapt well.

The remediation pass addressed the commercial-release blockers across supported screen sizes: mobile media workspaces, invalid interactive markup, chat/branch state integrity, overlay behavior, keyboard/accessibility support, local-first font behavior, and the highest-value commercial workflow gaps.

**Release recommendation:** the audited UI milestone is ready for broader product QA. Retain the isolated Playwright remediation suite as a release gate and continue tracking the existing Vite main-chunk warning as performance work.

## What was inspected

- Chat landing, new conversation, composer, model selector, projects, conversation history, and global navigation
- Search, Usage, Prompt Templates, Plugins, Evaluation Harness, File Library entry points, Tools, and Settings
- Image Studio empty state, prompt controls, generated-image canvas, history, and edit/mask mode
- Music Studio session list, generation controls, playback/result, history, and asset metadata
- Video Studio projects, generation form, preview, history, and outputs
- Video Edit Studio media bin, preview, timeline, inspector, header controls, and mobile behavior
- Desktop and mobile overlay/focus behavior, console output, control sizes, overflow, and semantic accessibility tree

## P0 — release blockers

### 1. Image Edit is unusable at 390px

**Observed:** after opening an existing image, switching to Edit, and opening Canvas, the centered toolbar is wider than the viewport and is clipped by an `overflow-hidden` canvas. Brush and Eraser were outside the left edge; Fit and Download were outside the right edge. There was no horizontally scrollable toolbar. The canvas then displayed only a narrow top strip of the image with the rest of the working area black.

Playwright measured four inaccessible controls:

- Brush: x = -58 to -26
- Eraser: x = -24 to 8
- Fit: x = 375 to 407
- Download: x = 416 to 448

**Implementation evidence:** `frontend/src/components/image/CanvasToolbar.tsx:31-32` centers a single unwrapped row; edit controls use 32 × 32px targets at `:35-145`.

**Correction:** replace the centered absolute toolbar on narrow screens with an edge-constrained, horizontally scrollable toolbar or bottom sheet. Keep primary tools visible, move secondary actions to an overflow menu, use at least 44 × 44px touch targets, and explicitly fit/recalculate the canvas when entering edit mode or changing viewport size.

### 2. Music Studio hides its generation and history interfaces on mobile

**Observed:** at 390px the left Music panel was only 151px high while its content was 1,665px tall. The Generate heading began below the visible panel at y = 420, forcing users to discover a very small nested scroll area before they can reach the core action. The History panel received only 38px of visible height for 420px of content; its overflow was visible rather than scrollable and the root shell clipped the rest.

**Implementation evidence:** `frontend/src/components/music/MusicStudio.tsx:195-255` stacks desktop panels below `xl` while retaining large minimum heights (`520px` result and `420px` history).

**Correction:** use explicit mobile tabs such as **Create**, **Result**, and **History**, with one independently scrollable active panel. Do not stack three desktop workspaces inside an `overflow-hidden` app shell.

### 3. Video Studio collapses its primary workflow into tiny scrollports on mobile

**Observed:** at 390px the Projects/Create panel was 200px high for 1,311px of content. The Create Video form began below the initial visible area. The output/preview panel received only 24px of visible height for 584px of content, while Generation History occupied most of the screen.

**Implementation evidence:** the three-pane desktop workspace changes to a vertical stack at `frontend/src/components/video/VideoStudio.tsx:596-597` without a mobile navigation model.

**Correction:** introduce mobile workspace tabs (**Create**, **Preview**, **History**, **Outputs**) and preserve the current project header. Give every tab a full-height scroll container and keep Generate/Cancel actions sticky within the Create tab.

### 4. Video Edit exposes a desktop timeline inside a mobile viewport

**Observed:** the 390px editor showed overlapping tool-rail labels, wrapped status-bar values, clipped inspector navigation, and a 1,250px timeline toolbar inside a 372px horizontal scroll area. Sixteen interactive controls were initially off-screen, including Redo, Marker, Duplicate, Play, all zoom controls, Snap, Save, and Shortcuts. The scroll area had no strong affordance that more controls existed.

**Implementation evidence:** `frontend/src/components/video/VideoEditStudio.tsx:270-400` stacks full desktop regions below `xl`; the center retains a large preview/timeline composition.

**Correction:** provide a purpose-built mobile editor mode. Recommended structure: preview first, compact transport controls, simplified single-track timeline, and bottom sheets for Media and Inspector. Keep the existing full editor behind a documented desktop/tablet minimum width.

### 5. Conversation switching can pair a new title with the previous transcript

**Source-confirmed risk:** the sidebar changes `activeId` before fetching (`frontend/src/components/Sidebar.tsx:220-230`), the message store retains old messages while loading (`frontend/src/stores/index.ts:170-183`), and `ChatView` does not gate rendering on message loading (`frontend/src/components/ChatView.tsx:147-154`). Message actions then use the new conversation ID against the retained transcript (`ChatView.tsx:744-778`).

**Impact:** under latency or request failure, users can see the previous chat beneath a new title and potentially edit, branch, or regenerate against mismatched IDs.

**Correction:** track `loadedConversationId`, immediately replace the transcript with a skeleton when switching, and disable composer/message actions until the active and loaded IDs match. A per-conversation message cache is an even better long-term model.

### 6. Branch selection appears to discard the selected branch messages

**Source-confirmed defect:** `frontend/src/components/ChatView.tsx:633-643` fetches `branchApi.listMessages(...)`, discards the response, and then reloads the main conversation with `fetchMessages(activeId)`.

**Impact:** the UI can indicate a different branch while showing the main transcript, undermining a major advertised capability.

**Correction:** make message loading branch-aware (`conversationId + branchId`) and store/render the returned branch transcript directly. Add an end-to-end regression test that creates two branches with visibly different content.

## P1 — high-priority corrections

### 7. Image Studio reports no capable provider despite configured image models

**Observed:** Settings showed active Gemini and OpenRouter providers with selected default image models. The selected Image session nevertheless displayed “Add an image-capable provider in Settings” and disabled Generate/Edit.

**Correction:** centralize provider-capability resolution and expose a diagnostic reason: missing API key, provider disabled, unsupported model, failed model refresh, or permission issue. Add a **Test image generation** action in Settings. Avoid a generic setup message when an apparently valid image model is already selected.

### 8. Image History renders invalid nested buttons

**Observed:** opening an image session generated two React console errors: a `<button>` was rendered inside another `<button>`. This can cause hydration inconsistencies and unpredictable keyboard/click behavior.

**Implementation evidence:** the history row button begins at `frontend/src/components/image/ImageHistoryPanel.tsx:157`; the nested “Branch from here” button is at `:205-215`.

**Correction:** make the row a non-button container with one primary button plus a sibling branch action, or use a single button and expose branch through a context menu.

### 9. Mobile mode navigation does not close the drawer

**Observed:** selecting Chat, Image, Music, Video, or Edit changed the background content but left the sidebar covering most of the screen. Selecting a specific session did close it, so the behavior is inconsistent.

**Implementation evidence:** mode buttons only call `setAppMode(...)` at `frontend/src/components/Sidebar.tsx:511-564`.

**Correction:** after a mode change, close the sidebar when the mobile drawer variant is active. Preserve the current desktop behavior.

### 10. Model selector state persists behind global dialogs

**Observed:** opening the model selector and then Usage/Settings left the selector mounted behind the modal. Closing the modal revealed the still-open selector. Its full-screen dismissal layer also intercepted pointer actions and caused Playwright’s click on the model trigger to time out.

**Correction:** use one overlay manager with an explicit stack. Opening a modal should dismiss non-modal popovers. Escape should close the topmost overlay and focus should return to its trigger.

### 11. Search results lack the context needed to choose correctly

**Observed:** searching “Chicago” returned 50 message-level results. Cards repeated the role twice (“user” and “User”), showed an unexplained raw relevance score, and omitted the conversation title/project. Similar messages appeared several times with little differentiation.

**Correction:** show conversation title, project, role, date, highlighted match, and a short surrounding excerpt. Remove raw scores or translate them into a useful confidence/ranking explanation. Group multiple hits by conversation and add sort/filter controls that remain visible after search.

### 12. Reload loses the user’s working context

**Observed:** reloading returned to the marketing welcome screen even though the audit had an active new conversation and recent history. The app also uses a single state-driven `/` shell rather than URL routes (`frontend/src/main.tsx:10-16`, `frontend/src/App.tsx:423-427`).

**Correction:** restore the last active conversation/session/project and encode durable workspace state in routes, for example `/chat/:id`, `/image/:sessionId`, and `/video/:projectId/edit`. This enables refresh recovery, back/forward navigation, bookmarking, and support links.

### 13. Empty states are visually sparse and sometimes lack a primary action

**Observed:** Image Studio’s empty state is only an icon and “Select or create an image session to start.” The New action exists as a small icon in the hidden/mobile sidebar. Plugins and Eval also open into minimal empty states without guided examples.

**Correction:** every empty state should include one dominant next action, one sentence explaining value, and an optional sample/demo path. For Image: **Create image session**, **Open recent**, and **Try a sample prompt**.

### 14. Session/project navigation is duplicated inside studios

**Observed:** Music and Video display sessions/projects in both the global sidebar and the studio’s own left panel. At 1440px this creates four visual columns and reduces the working canvas. Long auto-generated names are repeatedly truncated and difficult to scan.

**Correction:** choose one navigation owner. On desktop, either make the global sidebar mode-only while a studio is open, or remove the studio’s duplicate list. Generate concise, editable titles; add thumbnails, status, sort/filter, and pin/favorite support.

### 15. Music playback has duplicate control surfaces

**Observed:** the Music result displayed the browser’s native audio control and a second custom row with Play, Loop, Volume, and Download. This creates two play buttons and two volume concepts with unclear synchronization.

**Correction:** use one branded player with accessible keyboard controls, waveform scrubbing, current/total time, loop, volume, and download. Fall back to native controls only if the custom player fails to initialize.

### 16. The interface is too dense for touch

**Observed:** on desktop Video Edit, 152 of 179 visible interactive elements were below 44px in at least one dimension; two visible buttons had no accessible name. At 390px, 136 of 162 controls were below 44px. Many are intentional 32px professional-editor controls, but they remain the same size on touch layouts.

**Correction:** define separate compact-desktop and touch density modes. Use at least 44px touch targets, 12px minimum metadata text, and keep icon-only controls labeled with visible tooltips plus accessible names.

## Accessibility and design-system corrections

### Keyboard and semantic interaction

- Conversation, image, music, and video rows are clickable `div` elements rather than links/buttons (`frontend/src/components/Sidebar.tsx:948-960`, `:1090-1102`, `:1194-1206`, `:1277-1289`). Their overflow buttons also appear unnamed in Playwright’s accessibility snapshot.
- Model Selector lacks complete combobox/listbox semantics and arrow-key behavior (`frontend/src/components/ModelSelector.tsx:129-188`). Escape did not reliably dismiss it during the audit.
- Resizable panel handles are mouse-only (`frontend/src/components/ResizablePanels.tsx:88-129`).
- Code-block Copy is revealed only on hover, making it undiscoverable on touch and keyboard (`frontend/src/components/MarkdownContent.tsx:30-37`).

**Correction:** use native interactive elements, `aria-current` for selected rows, a shared menu/combobox primitive, focusable `role="separator"` handles with arrow-key resizing, and `focus-within`/coarse-pointer styles.

### Modal and form consistency

- Settings and several overlays reimplement dialog behavior rather than using the focus-managed `DialogShell` (`frontend/src/components/SettingsPanel.tsx:84-159`, `frontend/src/components/common/AppDialog.tsx:10-35`).
- Labels are often visually adjacent but not associated with controls; settings switches are buttons without `role="switch"` or `aria-checked` (`SettingsPanel.tsx:946-988`, `:1076-1088`).
- Browser console reported API-key password fields outside a form.

**Correction:** route every overlay through one dialog primitive with focus trap, initial focus, Escape, return focus, inert background, and body scroll lock. Introduce shared Field, Switch, Select, and validation-message components.

### Status announcements and reduced motion

- Streaming, thinking, upload, and error state changes lack `aria-live`, `role="status"`, or `role="alert"` (`frontend/src/components/ChatView.tsx:935-983`, `:1075-1137`).
- No reduced-motion implementation was found despite infinite/entrance animations (`frontend/src/index.css:77-144`, `:475-485`, `:545-551`).

**Correction:** add live regions for progress and alerts for failures. Honor `prefers-reduced-motion` globally and configure Framer Motion accordingly.

### Theme consistency and contrast

- At least 52 components reference undefined `surface-light` or `background` theme utilities while the defined token set stops at `surfaceHover` (`frontend/src/index.css:15-19`, `frontend/src/theme/tokens.ts:14-20`).
- Cloud theme muted text is approximately 3.25:1 before additional opacity is applied (`frontend/src/theme/themes.ts:115-130`), below the normal-text contrast target.

**Correction:** complete the surface token scale for every theme, remove unresolved utility names, add a token-usage lint rule, and test text/background pairs automatically to WCAG AA.

## Trust, onboarding, and capability gaps

### Local-first claim is not technically exact

The welcome screen says “100% Local — Your data stays on your machine, always” (`frontend/src/components/WelcomeScreen.tsx:47-48`) while `frontend/index.html:9-11` requests Google Fonts. Even if message content stays local, the external request leaks connection metadata and fails offline.

**Correction:** self-host Inter or use the system font stack. Review all runtime network calls before retaining an absolute “100% Local” claim.

### Provider onboarding needs readiness checks

The Start Conversation action can create a chat without a usable provider (`frontend/src/components/Sidebar.tsx:233-242`), producing a dead-end composer. Settings show configuration but do not clearly communicate tested connectivity/capabilities.

**Commercial improvement:** provide a first-run checklist with provider connection tests, chat/image/music/video capability badges, default model validation, and a sample request that does not incur an unexpected charge.

### Loading, empty, and error states are conflated

The sidebar store exposes loading/error state but the sidebar renders empty-state CTAs from empty arrays without consuming those states (`frontend/src/stores/index.ts:8-12`, `frontend/src/components/Sidebar.tsx:651-858`).

**Correction:** standardize four states across all lists: loading skeleton, true empty state, stale data with refresh, and error with retry/details.

### Attachment limit copy is contradictory

The chat accepts up to 50MB (`frontend/src/components/ChatView.tsx:39`) while user-facing feedback says 10MB (`ChatView.tsx:361-365`).

**Correction:** derive validation and copy from one shared configuration value and show per-file rejection reasons.

## Commercial-grade product recommendations

### Navigation and information architecture

- Add real routes/deep links and restore the last workspace.
- Add a command palette that unifies studios, tools, projects, sessions, templates, and settings.
- Virtualize long conversation/session/history lists and add sort, filter, pin, archive, and bulk actions.
- Replace raw generated filenames/timestamps with editable human titles and visual thumbnails.

### Provider and model experience

- Enrich the model picker with capability badges (tools, vision, files, image, audio, video), context window, price, latency tier, free/paid, and configured/unavailable state.
- Add favorites, recents, recommended defaults, and “why unavailable” explanations.
- Prevent contradictory states such as an enabled web-search control beside a model labeled “No tools.”

### Evaluation and prompt management

- Replace the developer-only Eval form (provider text, model text, raw suite JSON) with a no-code case builder, model multi-select, scoring criteria, dataset import, side-by-side outputs, pass-rate/cost/latency summaries, and exportable reports.
- Add search, categories, tags, variables, preview/fill flow, duplication, and version history to Templates. Built-in templates should be cloned before editing.

### Plugins and governance

- Add a browsable plugin catalog, verification/signing status, permission review, update controls, enable/disable, health status, and per-plugin logs.
- For commercial/multi-user deployments, add roles, audit logs, policy controls, shared workspaces, and administrator-managed providers/secrets.

### Usage and cost controls

- Expand Usage beyond totals: provider/model breakdown, trend charts, per-project attribution, latency/error rates, budget thresholds, alerts, and export.
- Show estimated cost before expensive image/music/video generations and final actual cost afterward.

### Media workflow polish

- Add autosave timestamps, background job progress, retry/resume, cancel confirmation, and persistent notification history.
- Use a consistent asset card with thumbnail/waveform, human title, provider/model, status, cost, duration/dimensions, and cross-studio actions.
- Consolidate repeated action rows into a primary action plus context menu to reduce visual noise.

## Recommended implementation sequence

1. **Stabilize core behavior:** fix conversation/branch state integrity, invalid nested buttons, image provider detection, and overlay stacking.
2. **Choose a mobile strategy:** implement tabbed mobile studios or explicitly gate complex authoring by viewport. Fix Image Edit canvas/toolbar immediately.
3. **Build shared UI primitives:** Dialog, Popover, Menu, Combobox, Switch, Field, Status, Empty/Error state, and responsive Toolbar.
4. **Repair the design system:** complete theme tokens, contrast, typography, touch sizes, focus styles, and reduced motion.
5. **Polish commercial workflows:** onboarding/readiness, deep links/restoration, no-code Eval, plugin governance, model metadata, and cost controls.
6. **Add regression coverage:** keyboard-only flows, focus trapping, mobile screenshot tests at 390/768/1024px, overlay-stack tests, slow-network conversation switching, and branch transcript assertions.

## Acceptance criteria for the next UI milestone

- No essential controls are clipped or unreachable at declared supported widths.
- Chat title, transcript, branch, and action IDs always represent the same loaded state.
- All dialogs/popovers have predictable Escape, focus trap, return focus, and stack behavior.
- Core navigation and list rows work with keyboard and screen readers.
- No invalid nested interactive elements or console errors occur during the inspected flows.
- Loading, empty, error, and configured-but-unavailable states are distinct.
- Every studio has one clear primary action and restores the user’s last working context.
- Automated visual/accessibility coverage runs for dark/light themes and desktop/mobile breakpoints.

## Audit notes

- No provider generation requests, evaluations, uploads, deletes, exports, or other potentially billable/destructive actions were executed.
- The Playwright walkthrough used the Start Conversation CTA twice, creating two empty “New Conversation” records. They were left intact to avoid deleting user data without explicit approval.
