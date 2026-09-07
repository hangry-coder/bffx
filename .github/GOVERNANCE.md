# BFFX Project Governance

The BFFX project is committed to building a sustainable, welcoming, and high-performance open-source community. This document outlines the governance model, project roles, decision-making processes, and guidelines for contribution.

---

## 1. Project Roles

We define the following roles within the BFFX ecosystem:

### 1.1 Users
Users are members of the community who run the BFFX framework in local development, pilot projects, or production environments. Users are encouraged to report bugs, request features, and participate in community discussions.

### 1.2 Contributors
Contributors are community members who submit pull requests, improve documentation, write tutorials, or help triage issues. Anyone who makes a contribution to a BFFX repository is automatically a Contributor.

### 1.3 Maintainers
Maintainers are trusted contributors who have demonstrated a deep understanding of the BFFX codebase, architectural vision, and design values. Maintainers have write and merge privileges on the repository and are listed in [MAINTAINERS.md](MAINTAINERS.md).

---

## 2. Decision-Making Process

BFFX operates on a **Consensus-Seeking** model with a **Project Lead** steering override for critical architectural decisions.

### 2.1 Lazy Consensus
For minor patches, bug fixes, documentation improvements, and non-breaking changes, we use a *Lazy Consensus* model.
- A pull request can be merged once it receives at least one approving review from a Maintainer and passes all CI automated checks.
- If no objections are raised within 72 hours, the change is considered approved.

### 2.2 Active Consensus
For substantial modifications, feature additions, or changes that alter the public API surface or manifest schemas:
- An issue or RFC (Request for Comments) must be opened first to discuss the design.
- The proposal must receive explicit approval from at least two Maintainers.
- Any Maintainer can request to stall a merge if they believe the change requires further design review.

### 2.3 Technical Steering Override
In rare cases where maintainer consensus cannot be reached after constructive debate, the Project Lead (Jay Sadiq) acts as the final decision-maker to resolve technical stalemates and ensure cohesive framework evolution.

---

## 3. RFC (Request for Comments) Process

To propose significant changes (e.g., introducing a new primary storage engine, changing the wire protocol standard, or redefining the action hook execution pipeline):
1. **Submit an RFC**: Create a markdown proposal describing the problem, proposed solution, architectural impact, and alternative designs.
2. **Review Stage**: Share the RFC with the community via issues or discussions. Allow at least one week for maintainer and contributor feedback.
3. **Resolution**: The proposal is either approved, modified, postponed, or rejected based on technical consensus.
