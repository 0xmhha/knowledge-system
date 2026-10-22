"""Check the WI-V1 12-contract evidence rebinding review against its cited packets.

The review is an AI rebinding of stage 90-94 observations to the canonical
requirements; it never promotes a contract to satisfied or independently
approved. Every citation must match the cited packet's sealed manifest and the
summary values it quotes.
"""
import json
from wiv1_generation_boundary import ROOT, FOLLOWUPS, sha, read, require

PACKET = FOLLOWUPS/'wiv1-contract-evidence-v1'
REQUIREMENTS = FOLLOWUPS/'ticket-dev-gold-v1/private/requirements.json'
PREVIOUS = FOLLOWUPS/'wiv1-contract-consumer-review-v1/REVIEW.json'
# Packets a citation may name: the stage 90-94 observations and the latest four-role API run.
CITABLE = ('factory-native-overlap-v1', 'wiv1-generation-boundary-v1', 'wiv1-transport-timer-generation-v1',
           'wiv1-notify-transport-generation-v1', 'wiv1-progress-true-generation-v1', 'latest-four-role-api-v1')
TOOLS = ['scripts/wiv1_contract_evidence.py', 'scripts/test_wiv1_contract_evidence.py', 'scripts/wiv1_generation_boundary.py']


def inspect_review(review):
    requirements = read(ROOT/REQUIREMENTS)
    previous = {r['id']: r for r in read(ROOT/PREVIOUS)['requirements']}
    require(review['requirements_source'] == dict(path=str(REQUIREMENTS), sha256=sha((ROOT/REQUIREMENTS).read_bytes())), 'canonical requirements differ')
    require(review['previous_review'] == dict(path=str(PREVIOUS), sha256=sha((ROOT/PREVIOUS).read_bytes())), 'previous review differs')
    rows = review['requirements']
    require([r['id'] for r in rows] == [r['id'] for r in requirements] and review['reviewed_contracts'] == len(requirements) == 12, 'not the canonical 12 contracts in order')
    cited = set()
    for row, req in zip(rows, requirements):
        require((row['tier'], row['assertion'], row['unknown']) == (req['tier'], req['assertion'], req['unknown']), row['id']+': requirement text changed')
        require(row['previous_remaining'] == previous[row['id']]['remaining'], row['id']+': previous gap rewritten')
        require(row['full_requirement_satisfied'] is False and row['independent_approved'] is False, row['id']+': contract promoted')
        require(isinstance(row['covered_now'], str) and row['covered_now'].strip() and isinstance(row['remaining'], str) and row['remaining'].strip(), row['id']+': coverage or remaining gap missing')
        for ev in row['evidence']:
            require(ev['packet'] in CITABLE and ev['relevance'].strip(), row['id']+': uncitable packet '+ev['packet'])
            folder = ROOT/FOLLOWUPS/ev['packet']
            require(ev['manifest_sha256'] == sha((folder/'manifest.json').read_bytes()), row['id']+': cited manifest differs from '+ev['packet'])
            summary = read(folder/'SUMMARY.json')
            require(ev['summary_fields'] and all(k in summary and summary[k] == v for k, v in ev['summary_fields'].items()), row['id']+': cited value differs from '+ev['packet'])
            require(summary.get('formal_completed', 0) == 0 and summary.get('independent_approved', 0) == 0, row['id']+': cited packet claims acceptance')
            cited.add(ev['packet'])
    require(review['full_requirement_satisfied'] == 0 and review['full12_acceptance'] is False and review['installed_positive_WIV1_oracle'] is False, 'review promoted the contract set')
    require(review['formal_completed'] == review['independent_approved'] == review['FINAL'] == 0 and review['freeze'] is None and review['execution_allowed'] is False, 'acceptance boundary changed')
    require(review['anonymous_reviewer_role_status'] == '미정' and review['personal_names_required'] is False, 'reviewer role status changed')
    require(review['blocked_next'] and all(b['requires'].strip() for b in review['blocked_next']), 'independent prerequisites dropped')
    return dict(contracts=len(rows), citations=sum(len(r['evidence']) for r in rows), cited_packets=sorted(cited),
                contracts_without_new_evidence=[r['id'] for r in rows if not r['evidence']])


def seal(p):
    (p/'manifest.json').write_text(json.dumps(dict(tools={n: sha((ROOT/n).read_bytes()) for n in TOOLS}, artifacts={str(n.relative_to(p)): sha(n.read_bytes()) for n in sorted(p.rglob('*')) if n.is_file() and n.name != 'manifest.json'}), indent=2)+'\n')


def check(p=None):
    p = p or ROOT/PACKET
    try:
        m = read(p/'manifest.json')
        require(m['tools'] == {n: sha((ROOT/n).read_bytes()) for n in TOOLS}, 'verification source changed')
        require(m['artifacts'] == {str(n.relative_to(p)): sha(n.read_bytes()) for n in p.rglob('*') if n.is_file() and n.name != 'manifest.json'}, 'packet bytes changed')
        inspect_review(read(p/'REVIEW.json'))
        return []
    except (OSError, ValueError, KeyError, TypeError, IndexError) as error:
        return [str(error)]


if __name__ == '__main__':
    errors = check()
    if errors:
        raise SystemExit('\n'.join(errors))
    print('WI-V1 12-contract evidence rebinding: citations match sealed packets; satisfied0, independent/formal0')
