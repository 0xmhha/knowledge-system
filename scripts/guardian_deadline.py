"""Verify the deadline fixture diagnosis without inventing historical trace data."""
import copy
import json
import re
import subprocess
from pathlib import Path
import viewer_readiness as v
import coreml_readiness as prior

ROOT = v.ROOT
PACKET = Path('docs/spec-driven/operational-preparation/source-review-supplement/followups/guardian-deadline-diagnosis-v1')
BASE = '471dcddfbd668715b4cdf64e22c5527cd0bf7eec'
KIND = 'guardian_deadline_fixture_causality_native_DEV_not_independent_acceptance'
TOOLS = ['scripts/guardian_deadline.py', 'scripts/test_guardian_deadline.py', 'scripts/check-operational-acceptance.py', 'scripts/viewer_readiness.py', 'scripts/coreml_readiness.py', 'scripts/public_v2_readiness.py', 'scripts/run-wi4-target-subscription.py', 'scripts/verify-wi3-reasons-native.py']
TEST = 'TestGuardianAbsoluteDeadlineWorksWhileOwnerSocketStaysOpen'
GATE = 'TestGuardianDeadlineStopsWriterReleasedOnlyAfterReturn'
CHANGED = 'internal/setup/command_guard_test.go'


def match(pattern, text):
    m = re.search(pattern, text)
    v.require(m is not None, 'diagnostic ordering trace missing')
    return m


def classify(text):
    returned = match(r'GD return start=(\d+) deadline=(\d+) finish=(\d+).*marker_present=(true|false) stat_error=(.*)', text)
    before = int(match(r'GD kill_before pid=\d+ t=(\d+)', text)[1])
    after = int(match(r'GD kill_after pid=\d+ t=(\d+) error=<nil>', text)[1])
    marker = match(r'GD marker content=.*mtime=(\d+) existed_at_return=(true|false) mtime_after_return=(true|false)', text)
    stamp, finish = int(marker[1]), int(returned[3])
    v.require(returned[4] == marker[2] == 'true' and returned[5].strip() == '<nil>' and marker[3] == 'false', 'not a confirmed pre-return marker')
    v.require(before <= stamp <= after < finish and 'Killed: 9' in text, 'marker is outside observed group cancellation')
    return dict(kill_before=before, marker_mtime=stamp, kill_after=after, guardian_return=finish, marker_already_at_return=True, stat_error=False)


def command(p, expected=0):
    r = v.read(p/'receipt.json.gz')
    v.require(r['exit_code'] == expected and r['reaped'] and not any(r[k] for k in ('timeout', 'term_sent', 'kill_sent', 'remaining_live_group_members')), 'command outcome or cleanup differs')
    for s in ('stdout', 'stderr'):
        v.require(v.sha(v.raw(p/(s+'.gz'))) == r[s+'_sha256'], 'command stream changed')
    return r


def diagnostics(p, expected_failures):
    scope = v.read(p/'SCOPE.json.gz')
    v.require(scope['base_revision'] == BASE and scope['planned_groups'] == 8 and scope['repetitions'] == 50 and scope['total'] == 400, 'diagnostic population changed')
    v.require(scope['deadline_ms'] == 200 and scope['worker_sleep_seconds'] == 1 and scope['post_wait_seconds'] == 1.1 and scope['owner_socket_open'] and not scope['production_sources_modified'], 'original schedule weakened')
    for n, h in scope['original_sha256'].items():
        v.require(v.sha(subprocess.check_output(['git', 'show', BASE+':'+n], cwd=ROOT)) == h, 'diagnostic original source differs')
    totals = dict(pass_count=0, fail_count=0, skip_count=0)
    failures = []
    worker_ids = set()
    starts = 0
    for i in range(8):
        q = p/'groups'/str(i)
        result = v.read(q/'result.json.gz')
        r = command(q, 1 if result['tests']['fail'] else 0)
        v.require(r['argv'][-4:] == ['-test.v', '-test.run=^'+TEST+'$', '-test.count=50', '-test.timeout=180s'], 'diagnostic invocation changed')
        rows = v.events(v.raw(q/'stdout.gz'))
        counts = v.counts(rows)
        for row in rows:
            ids = re.findall(r'GD started pid=(\d+)', row.get('Output', ''))
            worker_ids.update(int(n) for n in ids)
            starts += len(ids)
        v.require(sum(counts.values()) == 50 and counts['skip'] == 0, 'diagnostic attempts dropped')
        v.require(result['tests'] == {k: counts[k] for k in ('pass', 'fail', 'skip')}, 'diagnostic result differs')
        for k in ('pass', 'fail', 'skip'):
            totals[k+'_count'] += counts[k]
        trace = []
        iteration = 0
        for row in rows:
            if row.get('Action') == 'run' and row.get('Test') == TEST:
                trace = []
                iteration += 1
            if row.get('Output'):
                trace.append(row['Output'])
            if row.get('Action') == 'fail' and row.get('Test'):
                v.require(row['Test'] == TEST, 'different diagnostic failure')
                failures.append(dict(group=i, iteration=iteration, **classify(''.join(trace))))
    v.require(totals == dict(pass_count=400-expected_failures, fail_count=expected_failures, skip_count=0), 'diagnostic denominator changed')
    v.require(v.read(p/'RESULT.json.gz')['source_preserved'], 'diagnostic source changed')
    census = v.read(p/'worker-groups-after.json.gz')
    v.require(census['recorded_worker_pgids'] == sorted(worker_ids) and census['remaining_live_members'] == [], 'recorded diagnostic worker group remains')
    return dict(**totals, workers_started=starts, deadline_elapsed_before_start=400-starts, recorded_worker_groups=len(worker_ids), remaining_live_worker_members=0, failure_orderings=failures)



def uninstrumented_runtime(p, source_binding, diagnostic):
    scope = v.read(p/'SCOPE.json.gz')
    v.require(scope['compiled_runtime_instrumentation'] is False and scope['legacy_test_source_revision'] == BASE, 'runtime logging was not removed')
    overlay = v.read(p/'overlay.json.gz')['Replace']
    v.require(len(overlay) == 1 and next(iter(overlay)).endswith('/'+CHANGED), 'runtime override remained in corroboration')
    v.require(v.raw(p/'overlay/internal/setup/command_guard_test.go.gz') == v.raw(diagnostic/'overlay/internal/setup/command_guard_test.go.gz'), 'corroboration changed observed test schedule')
    v.require(scope['original_sha256'] == {n: source_binding['source_sha256'][n] for n in scope['original_sha256']}, 'corroboration root source differs')
    counts = dict(pass_count=0, fail_count=0, skip_count=0)
    for i in range(8):
        q = p/'groups'/str(i)
        command(q)
        rows = v.events(v.raw(q/'stdout.gz'))
        c = v.counts(rows)
        v.require(c['pass'] == 50 and c['fail'] == c['skip'] == 0, 'uninstrumented corroboration population differs')
        for key in ('pass', 'fail', 'skip'):
            counts[key+'_count'] += c[key]
    v.require(v.read(p/'RESULT.json.gz')['source_preserved'], 'corroboration root changed')
    return dict(**counts, runtime_logging=False, conclusion='no reproduction in this additional sample; not proof the original historical event is known')


def boundary(d):
    v.require(d['CoreML_excluded'] and not d['production_runtime_changed'], 'excluded feature or runtime change claimed')
    v.require(d['exact_historical_event_forensically_proven'] is False, 'missing historical trace invented')
    v.require(all(d[k] == 0 for k in ('actual_model_calls', 'actual_LLM_calls', 'paid_external_calls', 'formal_completed', 'independent_approved')), 'diagnosis promoted to model or independent acceptance')


def derive(p):
    b = v.read(p/'source-binding.json')
    v.require(b['base_revision'] == BASE and b['changed_native_files'] == [CHANGED], 'native change is not test-only')
    parent = p.parent/'coreml-transport-v1'
    pm = v.read(parent/'manifest.json')
    v.require(v.sha((parent/'manifest.json').read_bytes()) == v.sha(subprocess.check_output(['git', 'show', BASE+':'+str((parent/'manifest.json').relative_to(ROOT))], cwd=ROOT)), 'historical parent manifest changed')
    v.require(pm['artifacts'] == v.inventory(parent), 'historical parent artifacts changed')
    for n, h in pm['tools'].items():
        v.require(v.sha(subprocess.check_output(['git', 'show', BASE+':'+n], cwd=ROOT)) == h, 'historical verifier source changed')
    v.require(v.read(parent/'SUMMARY.json') == prior.derive(parent), 'historical result changed')
    old = v.read(parent/'source-binding.json')
    v.require(set(b['source_sha256']) == set(old['source_sha256']), 'native source population changed')
    v.require([n for n in b['source_sha256'] if b['source_sha256'][n] != old['source_sha256'][n]] == [CHANGED], 'production inputs changed')
    v.require(b['approved_input_sha256'] == old['approved_input_sha256'] and b['generated_input_sha256'] == old['generated_input_sha256'], 'approved input or optional UI changed')
    original = diagnostics(p/'diagnosis', 2)
    corrected = diagnostics(p/'single-change-control', 0)
    corroboration = uninstrumented_runtime(p/'uninstrumented-runtime', b, p/'diagnosis')
    left = p/'diagnosis/overlay/internal/setup'
    right = p/'single-change-control/overlay/internal/setup'
    for n in ('runner.go', 'command_guard.go', 'guardian_testmain_test.go'):
        v.require(v.raw(left/(n+'.gz')) == v.raw(right/(n+'.gz')), 'control changed runtime instrumentation')
    first = v.raw(left/'command_guard_test.go.gz')
    v.require(first.count(b'`sleep 1; printf late > "$1"`') == 1 and first.replace(b'`sleep 1; printf late > "$1"`', b'`sleep 1 && printf late > "$1"`') == v.raw(right/'command_guard_test.go.gz'), 'control is not exactly one fixture change')
    focus = {}
    scope = v.read(p/'focus/SCOPE.json.gz')
    v.require(scope['source_sha256'] == {n: b['source_sha256'][n] for n in scope['source_sha256']}, 'focused source differs')
    for variant in ('general', 'race'):
        command(p/'focus'/variant)
        rows = v.events(v.raw(p/'focus'/variant/'stdout.gz'))
        counts = v.counts(rows)
        v.require(counts['pass'] == 27 and counts['fail'] == counts['skip'] == 0, 'focused lifecycle population changed')
        v.require(all(sum(x.get('Action') == 'pass' and x.get('Test') == name for x in rows) == 3 for name in (TEST, GATE)), 'deadline or post-return gate omitted')
        focus[variant] = dict(counts)
    command(p/'mutant/run', 1)
    mutant_scope = v.read(p/'mutant/SCOPE.json.gz')
    v.require(mutant_scope['original_sha256'] == {n: b['source_sha256'][n] for n in mutant_scope['original_sha256']}, 'mutant source differs from corrected fixture')
    for n, h in mutant_scope['overlay_sha256'].items():
        v.require(v.sha(v.raw(p/'mutant'/(n+'.gz'))) == h, 'mutant overlay bytes differ')
    mutant = v.read(p/'mutant/RESULT.json.gz')
    v.require(mutant['mutant_rejected'] and mutant['real_post_return_write_detected'] and mutant['remaining_live_members'] == [] and mutant['production_sources_preserved'], 'post-return mutant not rejected/cleaned')
    rows = v.events(v.raw(p/'mutant/run/stdout.gz'))
    v.require([x['Test'] for x in rows if x.get('Action') == 'fail' and x.get('Test')] == [GATE], 'mutant failure differs')
    v.require('deadline left a post-return writer:' in ''.join(x.get('Output', '') for x in rows), 'mutant failed for another reason')
    recovery = v.read(p/'attempts/initial-mutant/RECOVERY-RESULT.json.gz')
    v.require(len(recovery) == 1 and recovery[0]['remaining_live_members'] == 0 and recovery[0]['payload']['late'] == 'late', 'initial mutant recovery missing')
    receipts = v.read(p/'native/receipts.json.gz')
    v.require([r['label'] for r in receipts] == ['build', 'test', 'race', 'lint', 'build-bins', 'build-runtime', 'build-info'], 'whole native checks incomplete')
    native = {}
    for r in receipts:
        v.require(r['exit_code'] == 0 and r['reaped'] and not any(r[k] for k in ('timeout', 'term_sent', 'kill_sent')) and r['source_head'] == BASE and r['source_binding_sha256'] == v.sha((p/'source-binding.json').read_bytes()), 'native command or identity failed')
        streams = {s: v.raw(p/'native'/(r['label']+'.'+s+'.gz')) for s in ('stdout', 'stderr')}
        v.require(all(v.sha(streams[s]) == r[s+'_sha256'] for s in streams), 'native stream changed')
        if r['label'] in ('test', 'race'):
            expected = ['go', 'test']+(['-race'] if r['label'] == 'race' else [])+['-json', '-count=1', '-timeout='+('20m' if r['label'] == 'race' else '18m'), './...']
            v.require(r['argv'] == expected, 'whole native test narrowed')
            rows = v.events(streams['stdout'])
            native[r['label']] = prior.previous.native_population(rows, v.events(v.raw(parent/'native'/(r['label']+'.stdout.gz'))))
            v.require(r['tests'] == native[r['label']] == {'pass': 4273, 'fail': 0, 'skip': 18}, 'native counts changed')
            v.require(any(x.get('Test') == GATE and x.get('Action') == 'pass' for x in rows), 'post-return regression missing')
        if r['label'] == 'lint':
            v.require(all(x in streams['stdout'].decode() for x in ('0 issues.', 'go vet ./...', 'documentation: OK')), 'fmt/vet/lint/docs missing')
    v.process_groups(receipts, v.read(p/'native/process-groups.json.gz'))
    tools = v.read(p/'native/TOOLS-BEFORE-LINT.json.gz')
    after = v.read(p/'native/installed-tools.json.gz')
    v.require(tools['capture_before_lint'] and tools['go'].startswith('go version go1.26.8 ') and 'version 2.14.0 ' in tools['lint'] and all(tools[k] == after[k] for k in ('go', 'lint')), 'pinned tools changed')
    v.require(tools['lint_configuration_sha256'] == v.sha((ROOT/'.golangci.yml').read_bytes()), 'lint configuration changed')
    packages = v.read(p/'native/package-boundary-proof.json.gz')
    v.require(packages['previous'] == packages['current'] and not packages['removed_external_only'], 'native packages removed')
    binaries = {'cks': v.read(p/'native/binary.json.gz')['sha256']}
    for n, exe in (('graph', 'ckg'), ('vector', 'ckv')):
        for label in ('build-'+n+'-runtime', n+'-build-info'):
            r = v.read(p/'native'/(label+'-receipt.json.gz'))
            v.require(r['exit_code'] == 0 and r['reaped'] and r['remaining_live_group_members'] == 0 and r['source_head'] == BASE, 'extra binary build failed')
        binaries[exe] = v.read(p/'native'/(n+'-binary.json.gz'))['sha256']
    v.require(v.read(p/'candidate/source-binding.json.gz') == b and binaries == {n: v.read(p/'candidate'/(n+'-binary.json.gz'))['sha256'] for n in binaries}, 'candidate/native binary identity differs')
    d = dict(kind=KIND, original=original, single_change_control=corrected, uninstrumented_runtime=corroboration, focused=focus, mutant_rejected=True, native=native, native_source_inputs=len(b['source_sha256']), Go_files=sum(n.endswith('.go') for n in b['source_sha256']), changed_Go_files=1, production_runtime_changed=False, reproduced_cause='pre-return shell write during group cancellation misclassified as late writer', exact_historical_event_forensically_proven=False, historical_failure_preserved=True, post_return_contract_verified=True, CoreML_excluded=True, actual_model_calls=0, actual_LLM_calls=0, paid_external_calls=0, formal_completed=0, independent_approved=0)
    boundary(d)
    return d


def validate(root=ROOT, p=None):
    p = p or root/PACKET
    try:
        manifest = v.read(p/'manifest.json')
        v.require(manifest['artifacts'] == v.inventory(p), 'guardian evidence artifact population changed')
        v.require(manifest['tools'] == {n: v.sha((root/n).read_bytes()) for n in TOOLS}, 'guardian verifier changed')
        b = v.read(p/'source-binding.json')
        current = copy.deepcopy(b)
        current['source_sha256'].update({n: v.sha((root/n).read_bytes()) for n in TOOLS})
        current['native_source_binding_sha256'] = v.sha((p/'source-binding.json').read_bytes())
        v.require(current == v.read(p/'current-source-binding.json'), 'current source binding differs')
        for n, h in {**current['source_sha256'], **current['approved_input_sha256'], **current['generated_input_sha256']}.items():
            v.require(v.sha((root/n).read_bytes()) == h, 'current source input changed: '+n)
        names = subprocess.check_output(['git', 'ls-files', '--cached', '--others', '--exclude-standard', '*.go'], cwd=root, text=True).splitlines()
        v.require({n for n in names if n.startswith(('cmd/', 'internal/', 'pkg/'))} == {n for n in b['source_sha256'] if n.endswith('.go') and n.startswith(('cmd/', 'internal/', 'pkg/'))}, 'Go inventory changed')
        v.require(v.read(p/'SUMMARY.json') == derive(p), 'guardian diagnosis summary changed')
        return []
    except (ValueError, OSError, KeyError, TypeError) as e:
        return [str(e)]


def reference(root=ROOT):
    return dict(kind=KIND, manifest=str(PACKET/'manifest.json'), manifest_sha256=v.sha((root/PACKET/'manifest.json').read_bytes()))


def current_indices(root, protocol, ledger):
    try:
        docs = [v.read(root/n) for n in v.PATHS]
        docs[:2] = [protocol, ledger]
        ref = reference(root)
        work = Path('docs/spec-driven/REMAINING-13-EXECUTION.json')
        link = dict(path=str(work), sha256=v.sha((root/work).read_bytes()))
        active = v.read(root/work)
        excluded = next(r for r in active['optional_followups'] if r['id'] == 'COREML')
        v.require(active['optional_active_followups'] == 9 and active['optional_user_excluded_followups'] == 1 and excluded['active'] is False and excluded['status'] == 'user_excluded_not_complete', 'CoreML user exclusion changed')
        allowed = {'current_readiness_preparation', 'current_execution_state', 'active_remaining_worklist', 'adoption_acceptance_decision_proposal'}
        for n, d in zip(v.PATHS, docs):
            old = json.loads(subprocess.check_output(['git', 'show', BASE+':'+str(n)], cwd=root))
            v.require(all(d.get(k) == value for k, value in old.items() if k not in allowed), 'prior decision/gold changed')
            v.require(d['current_readiness_preparation'] == d['latest_guardian_deadline_followup'] == ref and d['current_execution_state'] == v.read(root/PACKET/'WORKLIST.json') and d['active_remaining_worklist'] == link, 'guardian current references differ')
        proposal = dict(path=str(v.PATHS[2]), sha256=v.sha((root/v.PATHS[2]).read_bytes()), status='D04_review_pending_local_native_DEV_not_independent_acceptance')
        v.require(all(docs[i]['adoption_acceptance_decision_proposal'] == proposal for i in (0, 1, 3)), 'proposal hash differs')
        return []
    except (ValueError, OSError, KeyError, TypeError) as e:
        return [str(e)]
