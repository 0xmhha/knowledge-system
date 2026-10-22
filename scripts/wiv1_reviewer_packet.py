"""Build and check the unfilled WI-V1 packet handed to the D04 gold reviewer.

The packet copies the AI drafts the reviewer must decide on (12 contracts,
the matching rules and the FINAL input requirements) and leaves every reviewer
field empty. The AI never fills a decision or the independence declaration;
the packet only records what the reviewer will be exposed to.
"""
import json
import sys
from wiv1_generation_boundary import ROOT, FOLLOWUPS, sha, read, require

SD = 'docs/spec-driven/'
PACKET = FOLLOWUPS/'wiv1-reviewer-packet-v1'
ASSIGNMENT = SD+'operational-preparation/reviewer-assignment-2026-10-22.json'
DEV_GOLD = FOLLOWUPS/'ticket-dev-gold-v1'
SOURCES = dict(requirements=str(DEV_GOLD/'private/requirements.json'),
               matching_contract=str(DEV_GOLD/'matching-contract.json'),
               dev_ticket=str(DEV_GOLD/'public/DEV-ticket.json'),
               contract_evidence_review=str(FOLLOWUPS/'wiv1-contract-evidence-v1/REVIEW.json'))
# The protocol hash moves with every ledger update, so only its WI-V1 input group is compared.
PROTOCOL = SD+'OPERATIONAL-ACCEPTANCE-PROTOCOL.json'
# Matching-contract keys inside the WI-V1 pilot; V2 belongs to WI-V2 and the rest are status, not rules.
MATCHING_KEYS = ('unit', 'source', 'V1', 'primary', 'secondary', 'API', 'mutation', 'candidate_decision', 'comparison', 'denominator', 'exposure')
DECISIONS = ['accept', 'revise', 'reject', 'defer']
REVIEWER = 'gold-reviewer'
CONTRACT_FIELDS = ('decision', 'revised_assertion', 'revised_unknown', 'primary_oracle', 'derived_oracle', 'notes')
RULE_FIELDS = ('decision', 'revised_rule', 'notes')
FINAL_FIELDS = ('fresh_FINAL_tickets', 'normal_controls', 'symptom_reintroduction_mutants', 'overfit_mutants', 'repetition_sufficiency', 'notes')
DECLARATION_FIELDS = ('reviewer_id', 'not_author_of_candidate_outputs', 'candidate_output_exposure_before_gold_finalization', 'ai_draft_exposure', 'conflicts', 'declared_date')
TOOLS = ['scripts/wiv1_reviewer_packet.py', 'scripts/test_wiv1_reviewer_packet.py']


def bound(path):
    return dict(path=path, sha256=sha((ROOT/path).read_bytes()))


def wiv1_group(protocol):
    return next(g for g in protocol['input_groups'] if g['id'] == 'WI-V1')


def build_review():
    requirements = read(ROOT/SOURCES['requirements'])
    evidence = {r['id']: r for r in read(ROOT/SOURCES['contract_evidence_review'])['requirements']}
    matching = read(ROOT/SOURCES['matching_contract'])
    group = wiv1_group(read(ROOT/PROTOCOL))
    contracts = [dict(id=r['id'], tier=r['tier'], ai_draft_assertion=r['assertion'], ai_draft_unknown=r['unknown'],
                      stage95_covered_now=evidence[r['id']]['covered_now'], stage95_remaining=evidence[r['id']]['remaining'],
                      evidence_citations=[dict(packet=e['packet'], manifest_sha256=e['manifest_sha256'], relevance=e['relevance']) for e in evidence[r['id']]['evidence']],
                      reviewer=dict.fromkeys(CONTRACT_FIELDS)) for r in requirements]
    return dict(schema_version=1, kind='WIV1_gold_reviewer_packet_unfilled_not_review', reviewer_id=REVIEWER, acceptor_id='owner-acceptor',
                assignment=bound(ASSIGNMENT), sources={k: bound(v) for k, v in SOURCES.items()},
                exposure_notice=dict(ai_drafts='every ai_draft_* field and matching ai_draft text was written by AI',
                                     candidate_outputs=sorted({c['packet'] for row in contracts for c in row['evidence_citations']}),
                                     dev_ticket='already exposed; DEV only, never FINAL'),
                decision_values=DECISIONS, contracts=contracts,
                matching_rules=[dict(key=k, ai_draft=matching[k], reviewer=dict.fromkeys(RULE_FIELDS)) for k in MATCHING_KEYS],
                excluded_matching_keys=dict(V2='WI-V2 is outside the WI-V1 pilot'),
                final_inputs=dict(requirements_proposal=group['requirements_proposal'], metrics_proposal=group['metrics_proposal'],
                                  reviewer=dict.fromkeys(FINAL_FIELDS)),
                reviewer_filled=False, independent_approved=0, formal_completed=0, formal_total=9, FINAL=0, freeze=None, execution_allowed=False)


def build_declaration():
    return dict(schema_version=1, kind='independence_declaration_template_unfilled', source='must_be_provided_by_gold-reviewer_not_AI',
                expected_reviewer_id=REVIEWER, assignment=bound(ASSIGNMENT), declaration=dict.fromkeys(DECLARATION_FIELDS))


def inspect(review, declaration):
    expected = build_review()
    require(review['sources'] == expected['sources'] and review['assignment'] == expected['assignment'], 'source binding differs')
    require(review['contracts'] == expected['contracts'], 'contract drafts differ from the canonical requirements and stage 95 review')
    require(len(review['contracts']) == 12, 'not the 12 canonical contracts')
    require(review['matching_rules'] == expected['matching_rules'] and review['final_inputs'] == expected['final_inputs'], 'matching or FINAL drafts differ')
    require(review == expected, 'packet differs from its sources')
    require(declaration == build_declaration(), 'independence declaration template filled or changed')
    return dict(contracts=len(review['contracts']), matching_rules=len(review['matching_rules']), cited_packets=len(review['exposure_notice']['candidate_outputs']))


def write(p, data):
    p.write_text(json.dumps(data, indent=2, ensure_ascii=False)+'\n')


def seal(p):
    write(p/'manifest.json', dict(tools={n: sha((ROOT/n).read_bytes()) for n in TOOLS},
                                  artifacts={str(n.relative_to(p)): sha(n.read_bytes()) for n in sorted(p.rglob('*')) if n.is_file() and n.name != 'manifest.json'}))


def check(p=None):
    p = p or ROOT/PACKET
    try:
        m = read(p/'manifest.json')
        require(m['tools'] == {n: sha((ROOT/n).read_bytes()) for n in TOOLS}, 'verification source changed')
        require(m['artifacts'] == {str(n.relative_to(p)): sha(n.read_bytes()) for n in p.rglob('*') if n.is_file() and n.name != 'manifest.json'}, 'packet bytes changed')
        inspect(read(p/'REVIEW-PACKET.json'), read(p/'INDEPENDENCE-DECLARATION.json'))
        return []
    except (OSError, ValueError, KeyError, TypeError, StopIteration) as error:
        return [str(error)]


if __name__ == '__main__':
    if sys.argv[1:] == ['build']:
        (ROOT/PACKET).mkdir(parents=True, exist_ok=True)
        write(ROOT/PACKET/'REVIEW-PACKET.json', build_review())
        write(ROOT/PACKET/'INDEPENDENCE-DECLARATION.json', build_declaration())
        sys.exit(0)
    if sys.argv[1:] == ['seal']:
        seal(ROOT/PACKET)
        sys.exit(0)
    errors = check()
    if errors:
        raise SystemExit('\n'.join(errors))
    print('WI-V1 reviewer packet: 12 contracts, 11 matching rules, FINAL inputs unfilled; declaration unfilled; reviewer decisions0, formal0')
