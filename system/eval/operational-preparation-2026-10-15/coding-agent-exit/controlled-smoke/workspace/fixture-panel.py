import json,sys
p=json.load(sys.stdin)
eligible=[c for c in p['candidates'] if c['hard_eligible']]
best=min(eligible,key=lambda c:c['metrics']['changed_lines']['value'])['id'] if eligible else None
print(json.dumps({'schema_version':1,'judge_id':p['judge_id'],'role':p['role'],
'preferred_candidate':best,'votes':[{'candidate_id':c['id'],'verdict':'accept' if c['hard_eligible'] else 'reject',
'reason':'controlled fixture: use observed hard eligibility and edit size, not independent judgment',
'evidence_sha256':c['evidence_sha256']} for c in p['candidates']]}))
