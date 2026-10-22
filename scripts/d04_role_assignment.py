"""Check the D04 role assignment record against the protocol, proposal and worklist.

The record holds the roles the user assigned; it never declares the reviewer's
independence, approves F-05 or counts a formal acceptance. The protocol roles,
the D04 receipt and the worklist reference must all name the same record bytes.
"""
import copy
from wiv1_generation_boundary import ROOT, sha, read, require

SD = 'docs/spec-driven/'
RECORD = SD+'operational-preparation/reviewer-assignment-2026-10-22.json'
PREVIOUS = SD+'operational-preparation/reviewer-assignment-2026-10-16.json'
PROTOCOL = SD+'OPERATIONAL-ACCEPTANCE-PROTOCOL.json'
PROPOSAL = SD+'OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json'
WORKLIST = SD+'REMAINING-13-EXECUTION.json'
STATUS = 'roles_assigned_independence_declaration_pending'
REVIEWER, ACCEPTOR = 'gold-reviewer', 'owner-acceptor'
# Values the D04 fields held before the assignment. Earlier stage guards compare
# everything else in the protocol and proposal against their base revisions.
UNASSIGNED_ROLES = dict.fromkeys(('gold_author', 'independent_content_reviewer', 'final_scope_acceptor', 'execution_operator', 'independence_declaration'))
UNASSIGNED_F05 = 'awaiting_assignment_user_confirmed_unassigned'
UNASSIGNED_D04 = 'recommended_scope_approved_roles_unassigned'


def unassigned(doc):
    """Return a copy of a protocol or proposal with only the D04 assignment fields reverted."""
    doc = copy.deepcopy(doc)
    if 'freeze_requirements' in doc:
        doc['roles'] = dict(UNASSIGNED_ROLES)
        doc.pop('reviewer_assignment_update', None)
        next(r for r in doc['freeze_requirements'] if r['id'] == 'F-05')['preparation_status'] = UNASSIGNED_F05
    for decision in doc.get('decisions', []):
        if decision['id'] == 'D04':
            decision['status'] = UNASSIGNED_D04
            decision.pop('assignment_receipt', None)
    return doc


def inspect_record(record):
    require(record['origin'] == 'direct_human_user_message' and record['decision_id'] == 'D04' and record['selected_option'] == 'A_role_separation', 'not a user D04 option A record')
    require(record['supersedes_assignment_status_of'] == PREVIOUS and read(ROOT/PREVIOUS)['assignment_status'] == 'unassigned', 'previous assignment rewritten')
    require(record['assignment_status'] == STATUS, 'assignment status promoted')
    roles = record['roles']
    require(roles['independent_content_reviewer'] == REVIEWER and roles['gold_author']['final_authority'] == REVIEWER, 'gold reviewer differs')
    require(roles['gold_author']['draft_assistance'] == 'AI' and roles['execution_operator'] == dict(operator='AI', confirmed_by=ACCEPTOR), 'AI role widened')
    require(roles['final_scope_acceptor'] == ACCEPTOR and REVIEWER != ACCEPTOR, 'acceptor differs or merges with reviewer')
    require(roles['independence_declaration'] is None and record['independence_declaration_source'] == 'must_be_provided_by_gold-reviewer_not_AI', 'independence declared without the reviewer')
    require(record['real_name_recorded'] is False and record['personal_contact_recorded'] is False, 'personal identity recorded')
    require(record['pilot_scope'] == ['WI-V1'], 'pilot scope changed')
    require(record['freeze_approval'] is False and record['independent_acceptance'] is False and record['formal_completed'] == 0 and record['formal_total'] == 9, 'acceptance promoted')
    require(record['freeze'] is None and record['execution_allowed'] is False, 'freeze or execution enabled')
    return roles


def inspect(record, protocol, proposal, worklist):
    roles = inspect_record(record)
    ref = dict(path=RECORD, sha256=sha((ROOT/RECORD).read_bytes()), status=STATUS)
    policy = protocol['reviewer_identity_policy']
    require(policy['real_name_required'] is False and REVIEWER in policy['role_alias_examples'], 'reviewer identity policy changed')
    require(protocol['roles'] == roles and protocol['reviewer_assignment_update'] == ref, 'protocol roles differ from the record')
    f05 = next(r for r in protocol['freeze_requirements'] if r['id'] == 'F-05')
    require(f05['decision_status'] == 'pending' and f05['receipt'] is None, 'F-05 approved without independence declaration')
    require(protocol['freeze'] is None and protocol['execution_allowed'] is False and 'F-05' in protocol['blocked_by'], 'protocol freeze or execution enabled')
    d04 = next(d for d in proposal['decisions'] if d['id'] == 'D04')
    receipt = d04['assignment_receipt']
    require({k: receipt[k] for k in ref} == ref and receipt['text'] == record['user_message'], 'D04 receipt differs from the record')
    require(receipt['independence_declaration'] is None and receipt['formal_gold_review_or_acceptance_recorded'] is False, 'D04 receipt promoted')
    entry = worklist['d04_role_assignment']
    require({k: entry[k] for k in ref} == ref and entry['pilot_scope'] == record['pilot_scope'], 'worklist reference differs from the record')
    require(worklist['formal_completed'] == 0 and worklist['groups_completed'] == 0 and worklist['freeze'] is None and worklist['execution_allowed'] is False, 'worklist promoted')
    return dict(reviewer=REVIEWER, acceptor=ACCEPTOR, pilot=record['pilot_scope'], independence_declaration=None)


def check():
    try:
        inspect(read(ROOT/RECORD), read(ROOT/PROTOCOL), read(ROOT/PROPOSAL), read(ROOT/WORKLIST))
        return []
    except (OSError, ValueError, KeyError, TypeError, StopIteration) as error:
        return [str(error)]


if __name__ == '__main__':
    errors = check()
    if errors:
        raise SystemExit('\n'.join(errors))
    print('D04 role assignment: gold-reviewer/owner-acceptor recorded, WI-V1 pilot; independence declaration pending, F-05 pending, formal0')
