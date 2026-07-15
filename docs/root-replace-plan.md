# Root Replace 지원 계획

`patch`가 **컨테이너(root 메시지·리스트·맵) 전체를 하나의 값으로 교체**할 수 있도록 하는 기능의 설계·구현 계획과 진행 현황 문서.

> 상태 범례: ⬜ 대기 · 🟨 진행중 · ✅ 완료 · ❌ 취소/보류

---

## 1. 배경

`Entry`는 `path`(진입 경로) → `targets`(연산 대상 필드/인덱스/키) → `kind`(remove/test/insert/assign/move/copy/nest) 구조다.

`proto/diff/delta.proto`에는 이미 설계 의도가 주석으로 남아 있다:

```proto
message Entry {
  // If no path nor targets are specified, the op applies to the root.
  ...
}
```

그러나 실제 구현(`patchproto/{message,list,map}.go`)은 `len(targets) == 0`일 때 **`nest`만** 처리하고 나머지 kind는 전부 `return nil`(no-op)로 버린다.

```go
if len(targets) == 0 {
    if entry.WhichKind() == dpb.Entry_Nest_case {
        return o.PatchField(c, fd, entry.GetNest())
    }
    return nil   // ← assign/remove/test/insert 가 여기서 무시됨
}
```

**"root replace"** 는 이 빈자리를 채워, `targets`가 없을 때 path 끝에 도달한 컨테이너 **자신**에 연산을 적용하는 기능이다. `path`로 임의의 서브메시지에 도달할 수 있으므로 "root"는 최상위 메시지만이 아니라 **path로 지정한 임의의 컨테이너**로 일반화된다 (예: `path=[m_1], targets=[]` → `m_1` 전체 교체).

## 2. 설계 결정

- **새 oneof kind를 추가하지 않고 기존 `assign`을 root에서 재해석**한다.
  - 근거: (1) delta.proto의 기존 주석 의도와 일치, (2) JSON Patch의 `replace(path:"")`와 정합, (3) "필드에 assign = 필드를 값으로 설정"의 자연스러운 확장이 "root에 assign = 컨테이너를 값으로 설정".
- **proto 스키마 변경 불필요.** `Value.l`(ListValue)·`Struct`(repeated KeyValue) 가 이미 존재하므로, repeated/map 필드도 기존 타입으로 표현 가능하다. 확장은 순수하게 Go 인코딩/디코딩 로직에서만 일어난다 → 사용자 요청("struct 확장이 자연스러운 API를 만든다면 그렇게")에 부합.

### root에서의 kind별 의미

| kind | message root | list root | map root |
|------|--------------|-----------|----------|
| `assign` | **전체 교체**: 기존 필드 clear 후 값(Struct) 적용 | 리스트 전체를 ListValue로 교체 | 맵 전체를 Struct(키→값)로 교체 |
| `remove` | 전 필드 clear (빈 메시지) | `Truncate(0)` (빈 리스트) | 전 키 clear (빈 맵) |
| `test` | 메시지 전체를 값과 비교 | 리스트 전체 비교 | 맵 전체 비교 |
| `insert` | 메시지가 비어 있을 때만 채움 | append-all | 없는 키만 채움(merge) |
| `nest` | (기존) 같은 컨테이너로 재귀 | (기존) | (기존) |

## 3. 인코딩 설계 (Struct 확장)

root replace 대상 메시지가 repeated/map 필드를 가질 때 값이 유실되지 않도록, "메시지 ↔ Value/Struct" 양방향 변환을 확장한다. **현재 두 방향 모두 list/map 필드를 스킵**하고 있어 이 확장이 root replace의 핵심 선결 작업이다.

- **repeated 필드** → `KeyValue{key: 필드, value: Value.l(ListValue)}` — 각 원소를 element 디스크립터로 인코딩.
- **map 필드** → `KeyValue{key: 필드, value: Value.m(Struct)}` — 각 엔트리를 `KeyValue{key: 맵키(FieldSegment), value: 맵값}`으로 인코딩.
  - 문자열 키 → `FieldSegment.name`, 정수 키 → `FieldSegment.number`.
- 디코딩 시 `Value.m`(Struct)이 "중첩 메시지"인지 "맵 내용"인지는 **필드 디스크립터**(`fd.IsMap()` vs message kind)로 구분 → 모호성 없음.

## 4. 파일별 작업

| # | 파일 | 내용 | 상태 |
|---|------|------|------|
| P1 | `dpb/entry.go` | 인코딩: `protoMsgToStruct`가 list/map 필드도 인코딩. `protoListToValue`/`protoMapToValue` 추가. root-replace 빌더 헬퍼(`ValM` 재사용 + 편의 함수) | ✅ |
| P2 | `patchproto/patch.go` | 디코딩: `applyStructToMessage`가 list/map 필드 복원. `setMessageField` 헬퍼 추가. | ✅ |
| P3 | `patchproto/message.go` | message root: remove/test/assign(replace)/insert 처리 + cursor notify | ✅ |
| P4 | `patchproto/list.go` | list root: remove/test/assign(replace)/insert(append) 처리 | ✅ |
| P5 | `patchproto/map.go` | map root: remove/test/assign(replace)/insert(merge) 처리 | ✅ |
| P6 | `patchproto/*_test.go` | root replace/remove/test 테스트 (repeated/map 포함, path 경유 서브메시지, round-trip) | ✅ |
| P7 | `README.md` | root 연산 문서화 | ✅ |

## 5. 엣지 케이스 / 주의점

- **양방향 대칭성**: 인코딩(P1)과 디코딩(P2)의 list/map 처리가 정확히 대칭이어야 round-trip(`ValM(msg)` → root assign → `msg` 복원)이 성립. 검증 대상 1순위.
- **element/value 디스크립터**: 리스트 원소는 list `fd`(kind=원소 kind), 맵 값은 `fd.MapValue()`를 써서 디코딩해야 함.
- **enum/bytes/음수 정수 키/부동소수** 등 타입별 인코딩 정확성.
- **presence**: message root `insert`는 `c`가 이미 값이 있으면 no-op(암묵적 존재 필드 판단).
- **nil 값**: `Value.n`(Null)/무효 값 → clear 의미 유지.
- **cursor/hook**: root 연산의 before/after Frame은 연산 전 컨테이너 스냅샷(`proto.Clone`)과 이후 상태로 알림.
- **proto 재생성 불필요**: 스키마 미변경이므로 `buf generate` 불필요.
- **알려진 제약(설계상)**:
  - list/map root의 bulk 연산(replace/clear/append/merge)은 cursor/hook notify를 발생시키지 않음. message root는 clone 스냅샷으로 notify함. 핵심 요청인 "root replace"(message)는 완전 지원.
  - `path`로 **미설정** 컨테이너에 도달해 **변형(assign/insert/remove/nest)** 하면 이제 패닉 대신 **명확한 에러**를 반환함(자동 생성은 미지원 — 향후 개선 여지). 읽기(test)는 빈 컨테이너로 정상 동작. 부재 컨테이너를 만들려면 부모에서 필드 단위 assign 사용.
  - null 원소/값(`Value.n`)은 list/map 재구성 시 드롭됨(round-trip에는 영향 없음 — 인코딩은 null을 만들지 않음). 정수/불리언 map 키는 코드상 대칭 처리되나 샘플 proto에 없어 테스트로는 미검증(잔여 커버리지 갭).
- **README 전면 갱신 완료(후속 작업)**: 구세대 API로 stale했던 나머지 전체를 현재 API로 재작성.
  - 정정: `Diff`/`Patch`/`Patched`는 `dpb`가 아니라 **`patchproto`** 패키지. 연산 kind는 `remove/test/insert/assign/move/copy/nest`(구 `deleted/merged/scattered/swapped` 제거). 존재하지 않던 `no_insert/no_update` flags 절, `target.Fields`/`ref.Field` 빌더 절 삭제.
  - 재작성 구성: 2-패키지 소개 · Quick start · Concepts(Delta/Entry, Diff 동작, Patch) · Addressing(targets/path/values 표) · Operations(7종) · Root operations · JSON Patch interop(`dpb.FromJsonPatch`) · Hooks(`WithHook`) · Message-type resolution(`WithTypes`) · Planned features.
  - 모든 코드 예시를 실제 타입으로 컴파일 검증(임시 `_test`로 빌드 통과 후 삭제).
  - 서술 정확성을 코드와 대조(에이전트) → 5건 정정: (1) map nest는 값이 메시지일 필요 없음(한 단계 더 nest할 때만), (2) Diff의 repeated 메시지 원소 변경은 `nest`, (3) move/copy 캐스팅 과장 완화(enum/fixed source는 same-kind만), (4) insert는 presence 있는 필드만 "부재 시에만"(proto3 스칼라는 항상 설정), (5) test는 부재 필드는 통과하나 부재 map 키는 불가.

## 6. 검증

- `go test ./...` 전체 통과.
- 신규 테스트: message/list/map root의 assign·remove·test, repeated/map 포함 메시지 round-trip, path 경유 서브메시지 replace.
- 구현 후 다각도(정확성/대칭성/엣지) adversarial 리뷰 워크플로 1회.

---

## 진행 현황 (Progress Log)

| 단계 | 상태 | 비고 |
|------|------|------|
| 계획 문서 작성 | ✅ | 본 문서 |
| P1 인코딩 확장 (`dpb/entry.go`) | ✅ | `protoListToValue`/`protoMapToValue`/`mapKeyToFieldSegment` 추가, `ReplaceWith` 빌더, `ValM` list/map 반영 |
| P2 디코딩 확장 (`patchproto/patch.go`) | ✅ | `setMessageField`/`setListField`/`setMapField`/`isClearValue` 추가, `applyStructToMessage`에 types 스레딩, 기존 테스트 회귀 없음 |
| P3 message root (`message.go`) | ✅ | `patchMessageRoot` + `snapshotMessage`/`clearAllMessageFields`/`messageIsEmpty`, clone 스냅샷 notify |
| P4 list root (`list.go`) | ✅ | `patchListRoot`, assign=Truncate+append / insert=append / remove=Truncate / test=길이+원소비교. bulk 연산은 notify 생략(문서화) |
| P5 map root (`map.go`) | ✅ | `patchMapRoot`, assign=clearAll+set / insert=merge(없는 키만) / remove=clearAll / test=size+키·값비교 |
| P6 테스트 | ✅ | `patchproto/root_test.go` 신규: message/list/map root × assign·remove·test·insert, 스칼라 전 종류·repeated·map 메시지 round-trip, path 경유 서브메시지. `ValL` 헬퍼 추가. gofmt/vet 클린 |
| P7 README | ✅ | "Root operations" 절 추가(replace/clear/test/insert, 실제 동작 API 기준) |
| 검증(adversarial 리뷰) | ✅ | 6-lens 리뷰 워크플로 + 3라운드 재공격(총 300+ 벡터). 결함 5종 발견·전부 수정. 최종 잔여 패닉 0 |

### 검증에서 발견·수정한 결함 (전부 회귀 테스트 추가)

| # | 결함 | 발견 | 수정 위치 | 성격 |
|---|------|------|-----------|------|
| B1 | 빈 문자열 map 키 `""`가 round-trip에서 `"0"`으로 손상 | 자가 검증 | `map.go` `fieldSegmentToMapKey`: 문자열 키를 `HasName()`만으로 판정 | 신규 기능 버그 |
| B2 | Value kind ↔ 필드 kind 불일치 시 패닉/nil-deref (예: 메시지 리스트에 문자열 assign, 스칼라 맵에 메시지 값) | 리뷰 워크플로 (CONFIRMED×2) | `patch.go` `checkValueAssignable` 가드 추가 | 신규 경로가 노출한 잠재 결함 |
| B3 | mutating root 연산이 path로 **미설정** 컨테이너 진입 시 패닉 | 재공격 1 | `navigate.go` `mutate` 플래그 + unset 컨테이너 진입 시 에러 | 신규 기능 범위 |
| B4 | 필드 단위 assign/insert/test/move/copy가 **repeated/map target**에 적용 시 패닉 | 재공격 1 | `message.go` `singularOnly` 가드 | 기존 버그(하드닝) |
| B5 | move/copy **source**가 repeated/map 필드일 때 패닉 | 재공격 2 | `message.go` move/copy source 가드 | 기존 버그(하드닝) |
| B6 | `test`(읽기)가 path로 메시지 맵의 **미존재 키** 진입 시 패닉 | 재공격 3 | `navigate.go` `navigateMap`: 무효 값 `IsValid()` 검사 | 기존 버그(하드닝) |

- 품질 개선(정합성/중복): list/map 디코딩 중복을 `appendDpbValues`/`setMapEntries` 공유 헬퍼로 통합; list/map root의 null-test를 message root와 동일하게 "비어있음 검사"로 정합화.
- 최종: `go test ./...` + `-race` 전부 통과, gofmt/vet 클린, patchproto 테스트 케이스 158개.

### 로그
- 2026-07-15: 코드베이스 분석 완료, 계획 문서 작성. 설계 결정(assign=root replace, 스키마 미변경, Struct 인코딩 확장) 확정.
- 2026-07-15: P1~P7 구현 완료. `dpb.ReplaceWith`/`dpb.ValL` 빌더, 인코딩·디코딩 list/map 대칭 확장, 세 컨테이너 root 연산, 신규 `patchproto/root_test.go`. `go test ./...` 및 `-race` 통과. gofmt/vet 클린.
- 2026-07-15: **버그 수정** — 자가 검증 중 빈 문자열 map 키(`""`)가 round-trip에서 `"0"`으로 손상되는 문제 발견. `fieldSegmentToMapKey`의 문자열 키 판정을 `HasName() && GetName()!=""` → `HasName()`으로 수정(빈 이름도 유효한 문자열 키로 인정). 회귀 테스트 추가. (B1)
- 2026-07-15: **6-lens adversarial 리뷰 워크플로** 실행 → 패닉 2종(B2) 확인. `checkValueAssignable` 가드로 근본 수정(패닉→에러). 반박된 5건 중 중복 제거·null-test 정합성은 품질 개선으로 반영.
- 2026-07-15: **재공격 3라운드** — 각 라운드에서 잔여 패닉을 추가로 발견·수정: B3(미설정 컨테이너 변형), B4(repeated/map target), B5(move/copy source), B6(미존재 메시지 맵 키 읽기). 각 수정마다 회귀 테스트 추가. 최종 라운드에서 **83 벡터 전부 패닉 없음** 확인. 전체 스위트 + `-race` 통과.
