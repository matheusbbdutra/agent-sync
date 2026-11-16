# Diagram Craft (architecture diagrams)

> Load it when producing or reviewing an architecture diagram — C4, context, container, sequence or flow.
> Scope: what goes in, how to label, how not to mislead the reader. It does not cover filling in an ADR nor the review rubric.
> Citations use the `p.N` marker of the origin digest.

## 0. Which artifact to produce

Choose the artifact by the question the reader will ask, not by the tool you have open.

| Reader's question | Artifact | What does NOT go in | Source |
| --- | --- | --- | --- |
| who talks to the system and through which channel | context (C4) | class, table, internal queue | p.321 |
| what executes and where it runs | container (C4) | step-by-step data flow | p.321 |
| what the container is made of inside | component (C4) | deploy, infrastructure | p.321 |
| how a flow behaves, including on failure | sequence/behavior | static topology | p.84 |
| what is deployed in which environment | structure/deploy | sequence of calls | p.84 |

- One question, one artifact. Two questions in the same image is the case of §3 (p.28, p.84).
- If you do not know which question the reader asks, the problem is one of audience, not of diagram (p.28).

## 1. One level of abstraction per artifact

- Do not mix context, container, component and code in the same artifact. Two levels in the same drawing produce clutter and confusion from the audience's point of view (p.28).
- The separation rule applies to **all** diagram types, not only to C4 (p.28).
- If two levels are necessary, produce two artifacts linked to each other (§2) instead of one dense artifact (p.28, p.30).
- Quick test: if the reader needs to ignore half of the boxes to understand the message, there are two levels in the same drawing (p.28).
- Level of abstraction also applies to text inside the drawing: a class name in a context diagram is noise (p.28).

## 2. Explicit link between artifacts of the same system

- Make the correspondence visible and named: label, ID, hyperlink or labeled dashed box (p.30).
- Do not let the reader infer that "service X in diagram A" and "component X in diagram B" are the same — wrong inference is the default failure mode (p.30).
- The book asks for **explicit** representational consistency in diagrams and documentation, and justifies it by the audience's cognitive cost (p.35).
- Practical rule: same element, same name and same symbol in all artifacts of the set (p.35).
- Consequence for versioned docs: the set of diagrams is a single artifact with N views, not N independent artifacts (p.35).

## 3. Structure and behavior are different artifacts

- A structural diagram communicates **what and where**; a behavioral diagram communicates **how and to whom** (p.84).
- If the drawing answers both questions, split it in two. The message is lost when both coexist in the same image (p.84).
- In practice: topology/deploy in one artifact; flow, sequence and failure in another (p.84).
- Do not use the same notation for both purposes just because the tool allows it (p.84).

## 4. Relations: unidirectional and labeled with what flows

- One arrow per relation, labeled with what passes in that direction (p.71).
- A two-headed arrow only when the same process occurs in both directions, which is rare; when in doubt, use two arrows (p.72).
- An ambiguous bidirectional arrow loses the information of who calls whom — the information simply disappears from the drawing (p.72).
- Label with the data or the command that travels, not with "call" or "communication" (p.71).
- For request/response, the request follows the direction of the text and the response flows in the opposite direction (p.92).
- Mark what is synchronous and what is asynchronous in the drawing itself or in the legend (§5, §9).

## 5. Color, legend and legibility

- No meaning may exist **only** in color. For each meaning encoded in color, add a second channel: pattern, symbol or text (p.51, p.53).
- Include a legend whenever there is a non-obvious convention; if you decide not to include one, record the reason next to the artifact (p.58).
- Be explicit, not implicit: the legend declares what each channel means (p.58).
- Minimum font of 12 pt in any delivered or committed diagram (p.61).
- Color blindness, grayscale printing and a projector are the test scenario of the rule, not the exception (p.51, p.53).

Color and legibility verification, in order:

1. convert the drawing to grayscale: if the information disappears, the color was alone (p.51);
2. turn off the legend: if the reader gets the meaning wrong, the legend was necessary and was not there (p.58);
3. measure the font in the exported artifact, not in the editor (p.61);
4. show it to someone outside the project and ask what each color means (p.53).

## 6. Flow and layout follow the reading expectation

- Start at top-left (or middle-left); flow top to bottom and left to right (p.67).
- A layout against the reading expectation forces the reader to reconstruct the order — a cost paid on every reading (p.67, p.69).
- Landscape canvas in 16:9 or 16:10 proportion when the consumption is on screen (p.92).
- Request and response on the same arrow or in the same direction is a reading error, not an aesthetic one (p.92).
- Before delivering, show the drawing for 10 seconds to someone outside and ask where it starts (p.69).

- Keep the same reading direction in all artifacts of the set; inverting the axis between two diagrams charges the reader with the reassembly (p.67, p.69).
- Progress bar, step number and order implicit in the node names are alternatives when the flow is branched (p.69).

## 7. Formal notation and access cost

- Before choosing UML, ArchiMate or BPMN, check whether author **and** audience master the notation (p.78).
- If they do not, use simple notation (C4, boxes and lines) — the access cost does not pay for the rigor (p.79, p.82).
- It is not mandatory to use UML exactly as specified when the specification does not serve the goal (p.82).
- Formal notation limits who can create or update the diagram; the typical result is a one-person bottleneck and an outdated diagram (p.198).
- An icon is never the only information: the message must remain understandable if the icons are removed (p.77).
- Selection rule: notation is a communication tool, not a certificate of rigor (p.79, p.198).

## 8. Do not distort scale, baseline or element size

- Chart baseline always at zero; if zero is not possible, use a table (p.99).
- In side-by-side visuals, use the same scale or make the scale difference explicit and obvious to the audience (p.106).
- Keep elements of the same type at the same size in logical diagrams — size must not suggest a weight that does not exist (p.106).
- The symptom to avoid: the same series of numbers supporting two opposite conclusions (p.99, p.106).
- It applies to a metric dashboard and to a performance comparison report, not only to a diagram (p.99).

- In a deploy diagram, an element of the same type at a different size suggests a capacity or role that does not exist (p.106).
- Misleading composition is a defect class of its own: the drawing induces a conclusion that the data does not support (p.103).

## 9. C4: what to use and where it stops

- Start with a cheap and iterable artifact; only invest in a polished diagram when the design has stabilized (p.319).
- The stated reason: an ephemeral draft prevents excessive attachment to the artifact — "irrational artifact attachment" freezes a bad decision (p.319).
- To communicate, use the four C's: Context, Container, Component (and code) (p.321).
- Line convention: solid indicates synchronous communication, dotted indicates asynchronous (p.322).
- Limit stated by the book: C4 is less suitable for distributed architectures (p.322).
- Derived rule: in a distributed system, complement C4 with its own flow/sequence artifact (§3) instead of forcing everything into C4 (p.322).
- A diagram without a declared convention that induces wrong interpretation is worse than no diagram (p.322).
- C4 describes structure; the behavior of a specific flow requires its own artifact (§3) (p.322, p.84).
- Low-fidelity draft first: the initial goal is to decide, not to present (p.319).

## 10. Anti-patterns

- **Diagram that explains everything**: four levels in the same image, none legible (p.28).
- **Bidirectional arrows by default**: who calls whom is lost (p.72).
- **Meaning only in color**: part of the audience loses the entire information (p.51).
- **No legend**: implicit convention read differently by each reader (p.58).
- **Icon as the only information**: the drawing becomes a test of knowledge of cloud icons (p.77).
- **UML for rigor**: only whoever wrote it can maintain it; it becomes a bottleneck (p.198).
- **Chart with a cut baseline**: same series, two conclusions (p.99).
- **Attachment to the expensive artifact**: a diagram polished too early defends a decision instead of communicating it (p.319).
- **Wrong level for the question**: a component diagram answering a context question (p.321, p.84).
- **Inverted scale between artifacts**: two drawings of the same flow in opposite directions (p.67).

## 11. Checklist before delivering

- [ ] A single level of abstraction in the artifact (p.28).
- [ ] Correspondence with the other artifacts of the set is labeled or linked (p.30, p.35).
- [ ] Structure and behavior are in separate artifacts (p.84).
- [ ] Every arrow is unidirectional and labeled with what flows (p.71, p.72).
- [ ] No meaning depends only on color; legend present or reason recorded (p.51, p.58).
- [ ] Font >= 12 pt; icons redundant, not single carriers of information (p.61, p.77).
- [ ] Reading starts at top-left and follows the expectation (p.67).
- [ ] Synchronous/asynchronous is marked (solid line x dotted) (p.322).
- [ ] No scale or size induces a false conclusion (p.99, p.106).
- [ ] Notation chosen is mastered by whoever will maintain the diagram (p.78, p.198).

## 12. How to conduct the diagram review

- Review against the reader's declared question (§0), not against personal taste (p.84).
- The reviewer is someone from the audience, not the author: the author already knows what they meant (p.69).
- Fails on level of abstraction? Split it into two linked artifacts (§2) instead of deleting content (p.28, p.30).
- Fails on notation? Change the notation and keep the message; simplifying the formal one is allowed (p.79, p.82).
- Fails on color? Add a second channel (pattern, symbol, text) instead of changing the palette (p.51, p.53).
- No legend by conscious decision? Record the reason next to the artifact (p.58).
- Review early with an ephemeral draft and invest in the polish only after the design has stabilized (p.319).
- A diagram that fails twice in the same review is a sign that two artifacts are missing, not one (p.28).

## Sources

- **Communication Patterns — A Guide for Developers and Architects** (Jacqui Read), ch. 1 "Essentials" (p.25, p.28, p.30, p.35), ch. 2 "Reduzir carga cognitiva" [reducing cognitive load] (p.51, p.53, p.58, p.61), ch. 4 "Narrativa em diagramas" [narrative in diagrams] (p.63, p.67, p.69, p.71, p.72), ch. 5 "Notacao" [notation] (p.77, p.78, p.79, p.82, p.84), ch. 6 "Composicao" [composition] (p.92, p.99, p.103, p.106), ch. 11 "Just-in-Time Architecture" (p.198).
- **Fundamentals of Software Architecture — An Engineering Approach** (Mark Richards, Neal Ford), ch. 21 "Diagramming and Presenting" (p.319, p.321, p.322) — C4, solid/dotted convention and limit in distributed architecture, used in §9 and §11.
