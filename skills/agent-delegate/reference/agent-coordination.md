# Agent coordination

Reading criterion for a system with two or more agents, or with an explicit control plane:
which components exist, who controls the flow, who plans, who executes and where approval happens.
Outside this file: CLI/target choice and `print`/`session` mode (see this skill's `SKILL.md`) and the
Thought/Action/Observation cycle of a single session (`agent-react`).

## 1. Declare the mode and the approval point

"Mode" here is the book's agent taxonomy, not the `print`/`session` mode of `SKILL.md`.

- Choose one of the 4 modes explicitly and record it in the design: direct LLM, proxy (the LLM
  reformulates the request), agent with user approval before the tool call, or autonomous (p.25).
- In the approval mode, the checkpoint is **before** the tool call: "before making a call, the
  LLM requires user approval" (p.25).
- Treat the autonomous mode as a recorded risk decision, not as a default: it is the mode the book
  associates with "most ethical and safety concerns" (p.25-26).
- With more than one task in the flow, configure the permission cadence — every task or every N
  tasks (p.32-33). Without a declared cadence, the checkpoint exists in the design and not in the flow.
- Do not label as autonomous what only reformulates the prompt (proxy): the wrong label hides that
  there is no approval at any point (p.25).

## 2. Decompose the agent into 5 components

- Describe every agent as profile/persona, actions/tools, memory/knowledge,
  reasoning/evaluation and planning/feedback (p.27-28).
- The profile is the core: "At the core of all agents is the profile and persona" (p.27-28).
- A nonexistent component becomes an "absent" line in the spec. An agent reduced to a large system
  prompt has nowhere to evolve part by part (p.27-28).
- Keep reasoning/evaluation and planning/feedback as distinct spec items: the book draws them as
  separate components (p.31).

## 3. Control plane: behavior tree, not FSM

- Model the control as a tree with the node types and binary success/failure return: selector
  (fallback), sequence, condition, action, decorator and parallel (p.153-154).
- Reject a state machine for a flow that will grow: "FSMs aren't practical for agents because
  they don't scale well" (p.156).
- Use the book's second argument in the same decision: the tree separates the decision logic
  ("Decoupling of decision logic", p.156).
- Branching, retry and precondition become tree nodes, not `if` scattered in the executor: it is
  the same node table that covers these three functions (p.153-154).

## 4. Build the topology by back chaining, starting from the goal

- Follow the 5 steps in this order: start from the goal's behavior, list the actions that lead to
  it, derive each action's success conditions, choose the communication mode, and only then assemble
  the nodes (p.178).
- The communication mode is its own back-chaining step ("Determine the mode of communication",
  p.178) — it is not an implementation detail decided later.
- Start with fewer agents than intuition asks for: "It's better to start evaluating with
  insufficient assistants than with too many" (p.179).
- Testing and verification stay in distinct agents: "testing and verification are best done with
  different assistants" (p.179).
- Do not assemble the tree by trial: the book's method derives the nodes from the success
  conditions, not from hypotheses about how many agents the problem "deserves" (p.178).

## 5. Choose the conversation topology by what needs to converge

- Several personas that need to see the same history: group chat with shared messages and an
  explicit `max_round` (p.106 uses `max_round=20`).
- Isolated steps: nested or sequential — accepting that the serial handoff can alter the task or
  the desired result (p.105).
- Weigh the cost before preferring group chat: those flows are "substantially more verbose and token
  expensive" (p.107).
- Isolate in a new thread the node that suffers from noise: the verifier runs "on a different
  thread, essentially in closed room" (p.171-172).
- Use a shared thread only where iteration is the goal; isolation leaves each assistant's
  interaction in a concise conversation that is easier to debug (p.173).
- The siloed assistant consumes the new thread alone ("are the only consumers", p.176): do not
  expect mutual feedback from whoever runs isolated.

| Situation | Topology | Evidence |
| --- | --- | --- |
| Personas need to converge on the same artifact | group chat with manager and `max_round` | p.106 |
| Single step, no context dependency | nested/sequential | p.105 |
| Verification stage that cannot be contaminated | new thread (silo) | p.171-172, p.176 |
| Iteration and mutual correction are the goal | shared thread | p.173 |
| Cost per run already high and small team | sequential/isolated, not group chat | p.107 |

## 6. Replace serial human review with a critic with a turn cap

- Use a second agent as proxy/critic when the review needs to be automatic: the proxy
  "works to replace the required human feedback and evaluation" (p.93).
- Write the critic's criterion in the persona, not in the reviewer's genius: "Your task is to
  scrutinize code content for any harmful or substandard elements" (p.103).
- Limit the critic's nested chat with a short `max_turns` (p.104 uses `"max_turns": 1`); without a
  cap, the critic becomes a second production loop.

## 7. Planner contract

- Inject only three things: the list of available functions with description/args, the goal and the
  corrective feedback from previous plans (p.303).
- Require in the prompt: JSON, "Each subtask must be from within the [AVAILABLE FUNCTIONS] list. Do
  not use any functions that are not in the list" and "The plan should be as short as possible" (p.303).
- Do not send conversation history to the planner: the plan is built in isolation to focus on the
  goal, because planning prompts consume many tokens (p.304).

## 8. Execute the plan outside the LLM

- Interpret the plan JSON in a local executor and keep a results context per function
  (p.304-305); return that context to the LLM only to summarize/answer.
- Do not depend on the model orchestrating sequential calls: "OpenAI, Groq, and Azure OpenAI all
  support parallel actions but not sequential or planned actions" (p.299).
- Declared gain of the external executor: it reduces tokens and tool use, and the model does not
  need to support tools to use a planner (p.308).
- Do not resend the whole state at each step: the executor populates the context and the LLM
  receives the consolidated result (p.304-305).

## 9. Coordination: channel and request contract

- Sync to build rapport and generate ideas; async to report progress, collect feedback and
  disseminate information (p.264-265).
- What defines async is the response expectation, not the tool: "Whether communication is
  asynchronous does not depend on the tool used but rather on the expectations of the way the tool
  is used" (p.272-279).
- If an asynchronous exchange goes past 4 back-and-forths without converging, switch to synchronous (p.279).
- Every request to another agent makes explicit Who (who answers and who is only informed), What
  (deliverable and format), When (absolute deadline), Wah-wah (what happens if there is no answer)
  and Why (p.280-281, p.293).
- Absolute deadline with a time zone, never relative (p.293-295): "Using statements such as end of day
  tomorrow or asap is meaningless"; use the form "@Gino, can you please let me know by 4 p.m. (CET)
  this Friday" (p.293-295).
- Delta relative to the `SKILL.md` brief: there are objective/authority/validation; here what is
  missing from the typical brief is the absolute deadline and the Wah-wah (p.280-281).

## 10. Choose the model by declared criteria

- Record the decision on the book's 6 criteria: type/use case, training data, size,
  benchmark performance, cost and deployment speed (p.34-35).
- Re-evaluate the choice when cost or latency start to dominate the use case (p.34-35).
- Do not expect a threshold from the book: it lists the criteria and defines neither prioritization
  nor cutoff numbers (p.34-35, medium confidence unit in the digest).

## 11. Before adding an agent or a hierarchical layer

- Turn on tracing (tokens, cost, latency, tool calls) and track thought repetition: repetitive
  patterns indicate a poorly decisive agent (p.117).
- If the cost doubles with no quality gain, change the processing pattern, tasks and goals — do not
  add a manager: "it will cost over double what it would have without the agent manager. The
  output will also likely not be significantly better" (p.117).
- Put evaluation and feedback at the end of the flow, as a separate stage, to avoid feedback looping
  (p.318).

## Component -> review question

| Component | Question the reviewer asks | Evidence |
| --- | --- | --- |
| profile/persona | what is the role and the success criterion of this agent? | p.27-28 |
| actions/tools | which actions exist and what is the schema of each one? | p.27-28 |
| memory/knowledge | what does this agent remember between executions? | p.27-28 |
| reasoning/evaluation | how does it decide that the current step is good? | p.31 |
| planning/feedback | how does it replan after failing? | p.31 |

## Warning signs in the design

| Sign | Reading | Fix |
| --- | --- | --- |
| Agent mode not declared | it is not known where approval happens (p.25) | declare mode + cadence (p.32-33) |
| Control tree with dozens of `if` states | disguised FSM, does not scale (p.156) | rewrite as tree nodes (p.153-154) |
| Planner receiving the conversation history | inflated and expensive plan (p.304) | cut history and corrective feedback (p.303-304) |
| Model orchestrating sequential calls | support nonexistent in the cited providers (p.299) | local executor (p.304-305) |
| Request with "asap" | deadline without reference (p.293-295) | date/time with time zone (p.293-295) |
| New agent to solve high cost | cost doubles with no gain (p.117) | measure repetition and review tasks/goals (p.117) |

## Design review checklist

- [ ] Mode declared and approval point before the tool call, with cadence (p.25, p.32-33).
- [ ] 5 components named, with "absent" explicit where missing (p.27-28, p.31).
- [ ] Control plane as a tree with binary return; no FSM (p.153-154, p.156).
- [ ] Topology derived by back chaining, including the communication mode (p.178).
- [ ] Testing and verification in distinct agents and the sufficient minimum of agents (p.179).
- [ ] Planner without history, bound to the function list and to the shortest plan (p.303-304).
- [ ] Local executor of the plan, LLM only summarizes (p.304-305, p.308).
- [ ] Request with absolute deadline and Wah-wah; escalation to synchronous on the 4th round-trip (p.279,
  p.293-295).
- [ ] Cost and repetition measured before scaling the topology (p.117).

## Sources

- AI Agents in Action (Micheal Lanham) — ch. 1 (p.25-33: modes, 5 components, permission
  cadence); ch. 2 (p.34-35: 6 model criteria); ch. 4 (p.93-107: proxy/critic, nested vs
  group chat, manager cost; p.117: repetition and cost before scaling); ch. 6 (p.153-156:
  behavior tree nodes and the FSM limit; p.171-176: siloed vs shared thread; p.178-179: back
  chaining); ch. 11 (p.299-308: planner and external executor; p.318: feedback isolated at the end).
- Communication Patterns (Jacqui Read) — ch. 14 (p.264-279: sync vs async, definition by
  response expectation, escalation on the 4th round-trip); ch. 15 (p.280-295: 4 Ws + Why, absolute
  deadline).
- Scope note: the brief listed U1, U2, U10, U12, U21, U22 and U25; the requested topics (multi-agent
  topology and coordination, approval before the tool call) also required U3, U7, U8, U9, U11 and
  U24, which the digest itself maps to the `agent-delegate` reference (section 3 of the digest).
