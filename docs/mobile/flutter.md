# Flutter client layout and migration (Legacy → regenerated)

The framework generator (`pkg/generator/mobile/flutter.go`) emits a **`BFFXClientGenerated`** class into `clients/flutter/lib/bffx_client.gen.dart` plus a small **`bffx_config.gen.dart`**. Older projects sometimes carried a monolithic, hand-edited `bffx_client.gen.dart` with custom helpers mixed into generated code.

## Recommended pattern

1. **Treat generated output as read-only** — never edit `bffx_client.gen.dart` by hand after `bffx sync`.
2. **Put custom HTTP helpers and domain sugar in a sibling file** — for example `bffx_client.dart` that imports `bffx_client.gen.dart` and defines:

```dart
import 'bffx_client.gen.dart';

/// App-specific facade; safe to edit across syncs.
class BFFXClient extends BFFXClientGenerated {
  BFFXClient({super.baseUrl, super.getToken});

  Future<void> myTeamHelper() async {
    // delegates into generated GET/POST wrappers
  }
}
```

3. **Run `bffx sync` from the project root** whenever manifests change; resolve merge conflicts only in the wrapper file, not in `*.gen.dart`.

## Side-by-side: old vs new

| Old (fragile) | New (stable) |
|---|---|
| Helpers added inside `bffx_client.gen.dart` | Helpers live in `bffx_client.dart` (or `lib/core/bffx/bffx_client.dart`) extending `BFFXClientGenerated` |
| Regenerate overwrites custom code | Regenerate overwrites only `bffx_client.gen.dart` |
| Single class `BFFXClient` | `BFFXClientGenerated` (generated) + `BFFXClient` (your extension) |

## One-time migration checklist (Legacy repos)

1. Copy the generator’s `bffx_client.gen.dart` and `bffx_config.gen.dart` from a fresh `bffx sync` into your app tree (paths may differ; align imports with your `pubspec.yaml`).
2. Rename your previous client class to `BFFXClient` **extends** `BFFXClientGenerated`, moving any custom methods from the old generated file into the subclass file.
3. Replace imports across the app so UI code depends on your **`BFFXClient`** wrapper, not on `BFFXClientGenerated` directly.
4. Run `dart analyze` and fix any signature drift against the new generated methods.

The canonical runnable layout is exercised by **[`examples/notes`](../../examples/notes/README.md)** in this repository.

---

## ⚡ Client Cache Semantics

The generated BFFX client supports two primary fetching mechanisms: **`getWithSWR`** and **`request`**. Choosing the right method is critical for mobile performance, battery life, and data consistency.

### Decision Matrix: `getWithSWR` vs `request`

| Method | Best Used For | Network Behavior | Cache Behavior | UI Experience |
|:---|:---|:---|:---|:---|
| **`getWithSWR`** | Read-heavy UI, list views, catalogs, profile screens, bootstrap data. | Background HTTP GET with `If-None-Match: <etag>`. | Memory/Disk-backed. Instant cache hit, background refresh. | **Instant (0ms)**. UI renders old data first, then animates updates seamlessly. |
| **`request`** | Mutations (POST, PUT, DELETE), transaction confirmations, real-time metrics. | Immediate network request directly to server. | Bypasses local cache completely. | **Blocking**. UI shows loading state until network call completes. |

---

## 🛠 Extending Generated SDK Models

Because `BFFXClientGenerated` is replaced on every `bffx sync`, you should define custom model wrappers or custom action hooks in your hand-written `bffx_client.dart` wrapper:

```dart
// Sibling file: lib/bffx_client.dart
import 'dart:convert';
import 'package:http/http.dart' as http;
import 'bffx_client.gen.dart';

class BFFXClient extends BFFXClientGenerated {
  BFFXClient({super.baseUrl, super.getToken});

  /// Custom high-level helper with domain models
  Future<List<MyTaskModel>> fetchActiveTasks() async {
    final response = await request('/api/v1/tasks', queryParameters: {'status': 'active'});
    if (response.statusCode != 200) {
      throw Exception('Failed to load tasks');
    }
    final List<dynamic> data = jsonDecode(response.body);
    return data.map((json) => MyTaskModel.fromJson(json)).toList();
  }
}
```

