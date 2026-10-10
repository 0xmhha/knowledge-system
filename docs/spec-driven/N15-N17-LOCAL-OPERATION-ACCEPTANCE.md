# N15·N16·N17 로컬 개인 운영 수용

2026-10-10 · **N15 완료, N17 완료, N16 미완료(서명 키 조건)**. 전체12/18, 남은6개.

## 1. 왜 이 범위인가

이전 C1 단계는 "시험 preview 범위로 종료"였고, 실제 운영 결정은 없었다. 사용자는 이번에 knowledge-system을 **로컬 개인 운영**으로 쓰기로 정했다([범위](../../system/eval/b0-knowledge-system/refactoring-n15-n17-operating-scope-2026-10-10/scope-decision.json)). 결정한 사실은 다음과 같다([사실](../../system/eval/b0-knowledge-system/refactoring-n15-n17-operating-scope-2026-10-10/operating-facts.json)).

- 운영자·담당자: 사용자 1인
- 대상 호스트: 이 Mac
- 소비 프로젝트: `wemade/go-stablenet`
- 복구 목표: RTO 1시간, RPO 24시간
- 백업: 같은 Mac의 다른 디렉터리
- 신뢰: 로컬 소스 빌드를 commit과 binary SHA로 확인

팀·조직 운영, 외부 제3자 소비자, 조직 법무 검토, 원격 배포는 범위 밖이다. 이 결정은 push·PR·merge·배포 권한을 주지 않는다.

## 2. 무엇을 실행했는가

1. **설치.** 운영 경로 `auto-coding/knowledge-data/ks-operating/releases/<commit>/bin`에 release별 binary를 두고 SHA를 기록했다([목록](../../system/eval/b0-knowledge-system/refactoring-n15-n17-operating-scope-2026-10-10/release-binaries.json)). 기존 `knowledge-data` 자료는 건드리지 않았다.
2. **실제 소비자에서 결함 발견과 수정.** 이전 release(`985e22d8`)는 go-stablenet의 submodule 4개 때문에 committed 캡처를 거부했다. 제품을 수정했다([수정](./N15-GITLINK-CAPTURE.md), `e90fab1f`). 그래서 이번 upgrade는 "거부되던 release에서 수정된 release로 교체"다.
3. **실제 모델·규모 색인.** 수정 release와 bge-m3(digest 고정, 1024차원)로 go-stablenet HEAD `740526d0`을 색인했다. 결과는 다음과 같다.
   - 소요 29.5분, embedding HTTP 1,617회·입력 20,958개, 실패 0
   - 파일 2,227개, 청크 20,956개(canonical 18,696개), 데이터셋 1.2GB(그래프 809MB)
   - submodule 4개는 경로와 commit만 기록되었다. 원본 저장소는 바뀌지 않았다.
4. **소비자 질의.** 새 `cks mcp` stdio 서버에 MCP 클라이언트로 실제 질문 5개를 보냈다([결과](../../system/eval/b0-knowledge-system/refactoring-n15-n17-operating-scope-2026-10-10/consumer-v1-e90fab1f.json)). 모두 오류 없이 인용 7–11개를 돌려줬고, 인용은 HEAD에 결합되었다. 질의 1건은 11–18초였다. 시간의 대부분은 composer의 backend 검색 단계(stage1 4–11초, stage2 약 3.5초, stage3 1.5–3.5초)였다. 이 규모에서 archive 신원 검증 1회는 약 0.62초다. 정답 기준이 없는 확인이므로 품질 판정이 아니다.
5. **복구 훈련.** 현재 버전을 백업 디렉터리에 복사했다(4.3초, SHA 동일). 데이터셋 루트의 이름을 바꿔 손실을 흉내 냈고, 손실 중 질의는 실패했다. 백업을 새 루트에 복사하고 `cks rollback`으로 current와 reader lease를 만들었다(14.5초). 손실 후 첫 질의 성공까지 35.2초였고, 검증 질의 5개를 끝낸 시점은 93.7초였다. 백업이 손실된 버전과 같으므로 이번 훈련의 색인 손실은 0이다([기록](../../system/eval/b0-knowledge-system/refactoring-n15-n17-operating-scope-2026-10-10/recovery-drill.json)). 손실을 흉내 낸 원래 루트는 `dataset.lost-<시각>`으로 남겨 두었다.

## 3. 사용자 수용

사용자는 세 가지를 수용했다([기록](../../system/eval/b0-knowledge-system/refactoring-n15-n17-operating-scope-2026-10-10/acceptance-decisions.json)). N15-D는 지원·비용·제한 수용이며, 지연은 후속 개선으로 남긴다. N16은 신뢰·권한 결정을 기록했다. N17은 복구 훈련과 운영 정책을 수용했다. 선택지는 agent가 권고했고, 사용자의 활동 시간은 측정하지 않아 null이다.

## 4. 한계

- 질의 지연 11–18초는 이 규모에서 개선이 필요하다(N10과 연결).
- 데이터 갱신 upgrade(같은 데이터셋 위의 binary 교체, 소스 변경 뒤 재색인)는 go-stablenet이 바뀌지 않아 실행하지 않았다. 새 v6 신원은 이전 binary가 읽지 못할 수 있으므로, 이전 release로 binary를 되돌리는 경로는 지원하지 않는다.
- 백업이 같은 디스크에 있어 디스크 고장과 기기 분실에는 보호되지 않는다. 단일 호스트이므로 실패전환은 백업 버전을 current로 복원하는 것이며, 두 번째 호스트로의 전환은 없다. 키가 없으므로 키 복구는 해당 없음이다.
- 복구 훈련은 agent가 실행했고, 사용자는 결과를 보고 수용했다.

## 5. 수용 조건

- [x] N15-D 실제 모델·규모·운영 소비자 설치/upgrade와 지원·비용·제한 수용 판정(로컬 개인 범위).
- [x] N15-E 현재 소스·원자료·수용 판정·한계를 문서와 원장에 결합.
- [x] N17-C backup/failover 담당·RTO/RPO·키 복구(해당 없음)·network/전원 정책·대상 호스트 판정.
- [x] N17-D 실제 운영 데이터셋 복구 훈련과 시간·데이터 손실·사용자 수용.
- [x] N17-E 코드·원자료·한계·판정 결합과 N18 연결. mock 복구 시간을 RTO/RPO 판정에 쓰지 않음.

**N16(미완료):** 권한자(사용자), 신뢰 경로(commit과 binary SHA), 고지 범위(재배포 없음)는 결정해 기록했다. 그러나 worklist 조건의 "공개키 fingerprint"는 사용자가 서명 키를 쓰지 않기로 해서 충족되지 않는다. 키를 만들어 서명하면 완료할 수 있다.

**남은 전체6개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N16 신뢰·권한, N18 기본값·출시 판정.
