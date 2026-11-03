# Diagnosis — 스냅 동기화 노드의 Anzeon 영수증 effectiveGasPrice = null

## 1. Root cause

**대상 값:** `types.Receipt.EffectiveGasPrice` (`core/types/receipt.go:71`)

Anzeon 체인에서 effectiveGasPrice는 **읽을 때 다시 계산하지 않고, 실행 시점에 계산한 값을 DB에 저장해 두었다가 그대로 읽는** 방식이다. 그런데 스냅 동기화로 들어온 영수증에는 이 값을 넣어 주는 단계가 없다.

- **생산 (실행 경로에만 있음):** `core/state_processor.go:157-159`
  `if config.AnzeonEnabled() { receipt.EffectiveGasPrice = msg.GasPrice }`
  `msg.GasPrice`는 `TransactionToMessage`(`core/state_transition.go:165-191`)에서 만든다. 이때 `statedb.IsAuthorized(from)`가 거짓인 일반 계정은 헤더의 GasTip을, 허가 계정(검증자·민터)은 tx 자신의 GasTipCap을 써서 `min(tip + baseFee, feeCap)`을 계산한다. 티켓에 나온 "허가 계정과 일반 계정의 계산 방식이 다르다"가 바로 이 분기다.
- **저장:** `ReceiptForStorage.EncodeRLP`가 값이 nil이 아닐 때만 저장 RLP 끝에 선택 필드로 붙인다(`receipt.go:285-290`). `DecodeRLP`가 읽을 때 복원한다(`receipt.go:307`).
- **파생 (Anzeon에서는 건너뜀):** `Receipts.DeriveFields`의 `if !config.AnzeonEnabled() { rs[i].EffectiveGasPrice = ... }`(`receipt.go:395-397`). Anzeon에서는 계산 자체를 하지 않으므로 값은 전적으로 저장된 것에 의존한다.
- **깨진 edge (스냅 동기화 수집 → 저장):** 피어는 영수증을 합의 인코딩 `receiptRLP{status, cumGas, bloom, logs}`로 보낸다(`eth/protocols/eth/handlers.go:270-277` → `Receipt.EncodeRLP` `receipt.go:129-141`). 이 형식에는 EffectiveGasPrice가 없다. 받는 쪽은 `Receipt.DecodeRLP`로 해석하고(`handlers.go:490-497`), downloader가 이를 `InsertReceiptChain`에 넘긴다(`eth/downloader/downloader.go:1726-1731, 1743`). 그 결과 `writeLive`의 `rawdb.WriteReceipts`(`core/blockchain.go:1264`)나 `writeAncient`가 **EffectiveGasPrice가 nil인 채로** 저장한다. 실행하지 않는 경로이므로 값이 만들어지는 지점이 없다.
- **소비:** `GetReceiptsByHash`(`core/blockchain_reader.go:218-236`) → `rawdb.ReadReceipts`(`core/rawdb/accessors_chain.go:630-661`) → `DeriveFields`(Anzeon이라 건너뜀) → `marshalReceipt`의 `"effectiveGasPrice": (*hexutil.Big)(receipt.EffectiveGasPrice)`(`internal/ethapi/api.go:1871`)가 null을 내보낸다.

정리하면 Anzeon의 EffectiveGasPrice는 "실행 시 생산 → 저장 필드" 한 경로에만 의존한다. 실행 없이 영수증을 넣는 경로(스냅 동기화의 `InsertReceiptChain`)에서는 생산 edge가 없고, 읽기 쪽 파생도 Anzeon에서 꺼져 있어 대체 수단이 없다. Anzeon이 아닌 체인은 `DeriveFields`가 tx와 baseFee로 다시 계산하므로 증상이 없다. 이것이 "Anzeon에서만 발생"한다는 구별 특징을 설명한다.

**배제한 경쟁 가설**
- *"전체 동기화 노드는 메모리 receiptsCache 덕분에 값이 있다":* 배제했다. 실행 경로의 영수증은 `ReceiptForStorage`로 DB에 저장된다(`receipt.go:288`). 따라서 캐시가 비워지거나 노드를 재시작해도 값이 남는다. 캐시는 원인이 아니고 차이는 DB 저장값에 있다.
- *"marshalReceipt나 RPC 직렬화 문제":* 배제했다. 필드를 그대로 넘길 뿐이며, 상류에서 이미 nil이다.
- *"피어가 값을 빼고 보낸다(서빙 측 버그)":* 배제했다. EffectiveGasPrice는 p2p/합의 영수증 형식(`receiptRLP`)에 처음부터 없다. receiptsRoot(DeriveSha) 검증도 합의 형식으로만 한다. 서빙 측을 고쳐도 받는 쪽 디코더가 이 값을 버린다.

## 2. Evidence

| 지점 | file:line | 내용 |
|---|---|---|
| 생산 | `core/state_processor.go:157-159` | Anzeon일 때만 `receipt.EffectiveGasPrice = msg.GasPrice` |
| 생산 입력 | `core/state_transition.go:165-191` | `!statedb.IsAuthorized(from)`이면 헤더 GasTip, 허가 계정이면 tx tip; `msg.GasPrice = min(tip+baseFee, feeCap)` |
| 저장 인코딩 | `core/types/receipt.go:104-109, 285-290` | `storedReceiptRLP.EffectiveGasPrice rlp:"optional"`, nil이 아닐 때만 기록 |
| 저장 디코딩 | `core/types/receipt.go:307`, `core/rawdb/accessors_chain.go:690-695` | 저장값 복원 (rawdb 쪽 중복 정의) |
| 파생 건너뜀 | `core/types/receipt.go:395-397` | `if !config.AnzeonEnabled()`일 때만 계산 |
| p2p 형식 | `core/types/receipt.go:96-101, 129-141, 162-230` | 합의 `receiptRLP`에 EGP 없음 |
| 서빙 | `eth/protocols/eth/handlers.go:258-285` | `rlp.EncodeToBytes(types.Receipts)`, 즉 합의 형식 |
| 수신 | `eth/protocols/eth/handlers.go:490-497` | 합의 디코딩 + DeriveSha |
| 스냅 커밋 | `eth/downloader/downloader.go:1726-1731, 1743` | `InsertReceiptChain(blocks, receipts, ancientLimit)` |
| 저장 (스냅) | `core/blockchain.go:1089`, `1264` (writeLive), `1161~` (writeAncient → `rawdb.WriteAncientBlocks` accessors_chain.go:782) | nil인 EGP를 그대로 저장 |
| 읽기 | `core/blockchain_reader.go:218-236` → `core/rawdb/accessors_chain.go:630-661` | ReadReceipts → DeriveFields |
| 출력 | `internal/ethapi/api.go:1854-1871` | `marshalReceipt` → null |

stablenet-knowledge: health ok/serviceable, index `aa28927fb` = HEAD(fresh). `get_for_task` 결과로 `internal/ethapi/api.go:1525 effectiveGasPrice`가 `api.go:1412-1520`에서 호출되는 관계(called_by)와 graphql `EffectiveGasPrice`(graphql.go:289-313) → `Header.GasTip`(block.go:111-117) 호출 관계를 확인했다. `DeriveFields` 호출자는 `rawdb.ReadReceipts`(accessors_chain.go:656), `BlockChain.collectLogs`(blockchain.go:2110), `chain_makers.go:412` 세 곳이다. 변경 이력: `receipt.go`의 Anzeon 건너뛰기 분기는 `d61c9140d feat: implement transaction prioritization and fee policy (#29)`에서 들어왔다.

## 3. Affected sites

- **must_fix (깨진 edge):** 실행 없이 영수증을 저장하는 경로에서 EffectiveGasPrice를 채우거나, 읽기 시 Anzeon 파생으로 보완해야 한다. 구체적으로 다음 중 한 곳 이상이다.
  - `core/types/receipt.go:395-397` `Receipts.DeriveFields`: Anzeon일 때 저장값이 nil이면 대체 계산이 필요하다(허가 여부 판단 입력이 필요함).
  - `core/blockchain.go:1089~1305` `InsertReceiptChain` (writeLive `:1264`, writeAncient `:1161~`): 스냅 수집 시 저장 전에 값을 채우는 지점.
  - `core/rawdb/accessors_chain.go:630-661` `ReadReceipts`: DeriveFields에 헤더 GasTip이나 허가 여부 정보를 넘기려면 시그니처와 호출부가 바뀐다.
- **sibling 소비자 (같은 nil을 보게 되는 곳):** `core/blockchain.go:2110` `collectLogs`(EGP는 쓰지 않음, 영향 적음), `eth/filters`/`ethclient` 등 `GetReceipts` 계열 RPC(`eth_getBlockReceipts`도 같은 `marshalReceipt` 경로를 탄다), `core/chain_makers.go:412`(테스트 체인 생성용으로 실행 경로를 거치므로 영향 없음).
- **이미 nil로 저장된 기존 DB:** 수정 후에도 스냅 동기화된 노드의 과거 영수증에는 값이 없다. 읽기 시 파생(대체 계산)이 없으면 재동기화나 마이그레이션이 필요하다.

## 4. Confidence

**high.** 코드 경로가 결정적이고, 같은 관측(null)을 내는 경쟁 경로가 없다. 생산 지점은 `state_processor.go:158` 한 곳뿐이며 실행 경로 전용이다. 파생은 Anzeon에서 명시적으로 꺼져 있고, p2p 영수증 형식에 이 필드가 없다. 런타임 프로브는 돌리지 않았다(정적으로 후보가 하나로 좁혀져서 불필요하다고 판단).
**확신을 더 높이려면:** Anzeon genesis 체인에서 `InsertReceiptChain`으로 넣은 블록과 `InsertChain`으로 넣은 블록을 `GetReceiptsByHash`로 비교하는 L2 in-process 테스트를 만든다. 전자는 `EffectiveGasPrice == nil`, 후자는 non-nil이어야 한다. 또는 chainbench에서 snap-sync EN 노드와 full-sync 노드의 `eth_getTransactionReceipt` 결과를 비교한다.

## 5. Suggested direction

Anzeon 블록에 대해서는 실행 없이 들어온 영수증에도 EffectiveGasPrice를 결정적으로 채워야 한다. 가장 근본적인 방법은 `DeriveFields`(또는 `ReadReceipts`/`InsertReceiptChain`)에 Anzeon 파생 경로를 두는 것이다. 저장값이 nil이면 `min(tip + baseFee, feeCap)`을 다시 계산하되, tip은 헤더 `GasTip()`(WBFT extra)과 "발신자가 그 블록 시점에 허가 계정이었는가"로 고른다. 어려운 점은 허가 여부가 상태(`statedb.IsAuthorized`)에 의존하고, 스냅 동기화 노드는 과거 상태를 갖고 있지 않다는 것이다. 따라서 허가 여부를 상태 없이 판정할 근거를 찾아야 한다. 후보는 다음과 같다. ① 영수증이나 블록에 판정 정보가 남는지 확인한다. ② 허가 계정 집합을 시스템 컨트랙트 이벤트나 헤더에서 복원할 수 있는지 확인한다. ③ 최후의 수단으로 p2p 영수증에 EGP를 싣는다. 단 ③은 receiptsRoot로 검증되지 않으므로 신뢰 문제가 남는다. 이 결정을 Planner가 내려야 하며, 이미 nil로 저장된 기존 스냅 노드 DB에 대한 보완(읽기 시 파생 또는 재동기화)도 함께 다뤄야 한다.

### 부가 관찰 (in_scope:false, 별도 티켓 후보)
- `internal/ethapi/api.go:1525-1538` `effectiveGasPrice`와 `graphql/graphql.go:289-313` `EffectiveGasPrice`는 헤더 GasTip이 nil이 아니면 **발신자와 무관하게** 헤더 tip을 쓴다. 그래서 허가 계정이 보낸 tx의 `gasPrice`(eth_getTransaction*)와 graphql effectiveGasPrice가 실제 실행값(tx tip 기반)과 다르게 나올 수 있다. 증상이 다르며, 신뢰도는 medium이다.
