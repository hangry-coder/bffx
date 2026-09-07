package mobile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func GenerateTypeScriptClient(root string, reg *manifest.Registry) error {
	clientDir := filepath.Join(root, "clients", "typescript", "src")
	if err := os.MkdirAll(clientDir, 0o755); err != nil {
		return err
	}

	proj := reg.ProjectSpec()
	port := 8080
	apiPrefix := "/api/v1"
	if proj != nil {
		if proj.Runtime.Api.Port > 0 {
			port = proj.Runtime.Api.Port
		}
		if proj.App.ApiPrefix != "" {
			apiPrefix = proj.App.ApiPrefix
		}
	}

	var sb strings.Builder
	sb.WriteString("// GENERATED CODE - DO NOT MODIFY BY HAND\n")
	sb.WriteString("// BFFX TypeScript / JavaScript Client SDK with Realtime SSE & Type-Safe CRUD\n\n")

	// 1. Common Types & Interfaces
	sb.WriteString(`export type Unsubscribe = () => void;

export interface RealtimeEvent<T = any> {
  resource: string;
  action: 'created' | 'updated' | 'deleted';
  payload: T;
  timestamp: string;
}

export interface ListQueryParams {
  limit?: number;
  offset?: number;
  sort?: string;
  filter?: string;
}

export interface BFFXError {
  code: string;
  message: string;
  request_id?: string;
}

`)

	// 2. Resource Interfaces
	for _, res := range reg.Resources {
		spec := reg.ResourceSpec(res.Metadata.Name)
		name := res.Metadata.Name
		className := toCamelCase(name)

		sb.WriteString(fmt.Sprintf("export interface %s {\n", className))
		sb.WriteString("  id: string;\n")
		for _, f := range spec.Fields {
			if f.Name == "password" || f.Name == "otp_code" || strings.Contains(f.Name, "secret") {
				continue
			}
			tsType := "string"
			switch f.Type {
			case "bool":
				tsType = "boolean"
			case "int", "float":
				tsType = "number"
			}
			opt := "?"
			if f.Required {
				opt = ""
			}
			sb.WriteString(fmt.Sprintf("  %s%s: %s;\n", f.Name, opt, tsType))
		}
		sb.WriteString("}\n\n")
	}

	// 3. RecordService implementation
	sb.WriteString(`export class RecordService<T extends { id: string }> {
  constructor(
    private readonly client: BFFXClient,
    private readonly collectionName: string,
  ) {}

  private get path(): string {
    return ` + fmt.Sprintf("`%s/${this.collectionName}s`", apiPrefix) + `;
  }

  async list(params?: ListQueryParams): Promise<T[]> {
    const query = new URLSearchParams();
    if (params?.limit) query.set('limit', String(params.limit));
    if (params?.offset) query.set('offset', String(params.offset));
    if (params?.sort) query.set('sort', params.sort);
    if (params?.filter) query.set('filter', params.filter);
    const qs = query.toString();
    const url = qs ? ` + "`${this.path}?${qs}`" + ` : this.path;
    return this.client.request<T[]>(url, { method: 'GET' });
  }

  async get(id: string): Promise<T> {
    return this.client.request<T>(` + "`${this.path}/${encodeURIComponent(id)}`" + `, { method: 'GET' });
  }

  async create(data: Partial<T>): Promise<T> {
    return this.client.request<T>(this.path, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async update(id: string, data: Partial<T>): Promise<T> {
    return this.client.request<T>(` + "`${this.path}/${encodeURIComponent(id)}`" + `, {
      method: 'PATCH',
      body: JSON.stringify(data),
    });
  }

  async delete(id: string): Promise<boolean> {
    await this.client.request<void>(` + "`${this.path}/${encodeURIComponent(id)}`" + `, { method: 'DELETE' });
    return true;
  }

  subscribe(callback: (event: RealtimeEvent<T>) => void): Unsubscribe {
    const channel = this.collectionName;
    return this.client.realtime.subscribe(channel, (ev) => {
      if (ev.resource?.toLowerCase() === this.collectionName.toLowerCase() || ev.resource === this.collectionName) {
        callback(ev as RealtimeEvent<T>);
      }
    });
  }
}

`)

	// 4. RealtimeService
	sb.WriteString(`export class RealtimeService {
  constructor(private readonly client: BFFXClient) {}

  subscribe(channel: string, callback: (event: RealtimeEvent) => void): Unsubscribe {
    const url = ` + fmt.Sprintf("`${this.client.baseUrl}%s/realtime?channel=${encodeURIComponent(channel)}`", apiPrefix) + `;
    
    // In Browser or Node with EventSource polyfill
    let source: EventSource | null = null;
    let abortCtrl: AbortController | null = null;

    if (typeof EventSource !== 'undefined') {
      source = new EventSource(url);
      source.onmessage = (e) => {
        try {
          const data = JSON.parse(e.data);
          callback(data);
        } catch {
          // ignore heartbeats/comments
        }
      };
      source.onerror = () => {
        // EventSource will auto-reconnect
      };
      return () => {
        source?.close();
      };
    } else {
      // Fallback via fetch streaming for Node.js / React Native
      abortCtrl = new AbortController();
      (async () => {
        try {
          const res = await fetch(url, {
            headers: { 'Accept': 'text/event-stream' },
            signal: abortCtrl.signal,
          });
          if (!res.body) return;
          const reader = res.body.getReader();
          const decoder = new TextDecoder();
          let buffer = '';
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split('\n\n');
            buffer = lines.pop() || '';
            for (const chunk of lines) {
              for (const line of chunk.split('\n')) {
                if (line.startsWith('data: ')) {
                  try {
                    const parsed = JSON.parse(line.slice(6));
                    callback(parsed);
                  } catch {}
                }
              }
            }
          }
        } catch {}
      })();
      return () => {
        abortCtrl?.abort();
      };
    }
  }
}

`)

	// 5. ScreensService
	sb.WriteString(`export class ScreensService {
  constructor(private readonly client: BFFXClient) {}

  async get<T = any>(screenName: string, queryParams?: Record<string, string>): Promise<T> {
    const query = new URLSearchParams(queryParams).toString();
    const qs = query ? ` + fmt.Sprintf("`?${query}`") + ` : '';
    return this.client.request<T>(` + fmt.Sprintf("`%s/screens/${encodeURIComponent(screenName)}${qs}`", apiPrefix) + `, { method: 'GET' });
  }

  stream<T = any>(screenName: string, onData: (data: T) => void): Unsubscribe {
    const url = ` + fmt.Sprintf("`${this.client.baseUrl}%s/screens/${encodeURIComponent(screenName)}/stream`", apiPrefix) + `;
    if (typeof EventSource !== 'undefined') {
      const source = new EventSource(url);
      source.onmessage = (e) => {
        try {
          onData(JSON.parse(e.data));
        } catch {}
      };
      return () => source.close();
    }
    const abortCtrl = new AbortController();
    (async () => {
      try {
        const res = await fetch(url, { headers: { 'Accept': 'text/event-stream' }, signal: abortCtrl.signal });
        if (!res.body) return;
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const parts = buffer.split('\n\n');
          buffer = parts.pop() || '';
          for (const p of parts) {
            for (const line of p.split('\n')) {
              if (line.startsWith('data: ')) {
                try { onData(JSON.parse(line.slice(6))); } catch {}
              }
            }
          }
        }
      } catch {}
    })();
    return () => abortCtrl.abort();
  }
}

`)

	// 6. AuthService
	sb.WriteString(`export class AuthService {
  constructor(private readonly client: BFFXClient) {}

  async login(email: string, password: string): Promise<{ token: string; [k: string]: any }> {
    const res = await this.client.request<{ token: string }>(` + fmt.Sprintf("`%s/auth/login`", apiPrefix) + `, {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
    if (res.token) {
      this.client.setAuth(res.token);
    }
    return res;
  }

  async signup(email: string, password: string, name?: string): Promise<{ token?: string; [k: string]: any }> {
    const res = await this.client.request<{ token?: string }>(` + fmt.Sprintf("`%s/auth/signup`", apiPrefix) + `, {
      method: 'POST',
      body: JSON.stringify({ email, password, name }),
    });
    if (res.token) {
      this.client.setAuth(res.token);
    }
    return res;
  }

  async logout(): Promise<void> {
    await this.client.request<void>(` + fmt.Sprintf("`%s/auth/logout`", apiPrefix) + `, { method: 'POST' });
    this.client.setAuth('');
  }

  async anonymous(deviceId: string): Promise<{ token: string; [k: string]: any }> {
    const res = await this.client.request<{ token: string }>(` + fmt.Sprintf("`%s/auth/anonymous`", apiPrefix) + `, {
      method: 'POST',
      body: JSON.stringify({ device_id: deviceId }),
    });
    if (res.token) {
      this.client.setAuth(res.token);
    }
    return res;
  }
}

`)

	// 7. BFFXClient Class
	sb.WriteString("export class BFFXClient {\n")
	sb.WriteString(fmt.Sprintf("  readonly baseUrl: string;\n"))
	sb.WriteString("  private token?: string;\n")
	sb.WriteString("  private deviceId?: string;\n\n")
	sb.WriteString("  readonly auth: AuthService;\n")
	sb.WriteString("  readonly realtime: RealtimeService;\n")
	sb.WriteString("  readonly screens: ScreensService;\n\n")

	sb.WriteString("  constructor(baseUrl?: string) {\n")
	sb.WriteString(fmt.Sprintf("    this.baseUrl = (baseUrl || 'http://localhost:%d').replace(/\\/+$/, '');\n", port))
	sb.WriteString("    this.auth = new AuthService(this);\n")
	sb.WriteString("    this.realtime = new RealtimeService(this);\n")
	sb.WriteString("    this.screens = new ScreensService(this);\n")
	sb.WriteString("  }\n\n")

	sb.WriteString("  setAuth(token: string) { this.token = token; }\n")
	sb.WriteString("  setDeviceId(id: string) { this.deviceId = id; }\n\n")

	sb.WriteString("  collection<T extends { id: string } = any>(name: string): RecordService<T> {\n")
	sb.WriteString("    return new RecordService<T>(this, name);\n")
	sb.WriteString("  }\n\n")

	// Accessor properties for each resource
	for _, res := range reg.Resources {
		name := res.Metadata.Name
		field := toLowerCamelCase(name)
		className := toCamelCase(name)
		sb.WriteString(fmt.Sprintf("  get %ss(): RecordService<%s> {\n", field, className))
		sb.WriteString(fmt.Sprintf("    return this.collection<%s>('%s');\n", className, strings.ToLower(name)))
		sb.WriteString("  }\n\n")
	}

	sb.WriteString(`  async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(this.token ? { 'Authorization': ` + "`Bearer ${this.token}`" + ` } : {}),
      ...(this.deviceId ? { 'X-Device-ID': this.deviceId } : {}),
      ...(options.headers as Record<string, string> || {}),
    };

    const fullUrl = path.startsWith('http') ? path : ` + "`${this.baseUrl}${path}`" + `;
    const resp = await fetch(fullUrl, { ...options, headers });

    if (!resp.ok) {
      let errBody: any;
      try { errBody = await resp.json(); } catch { errBody = { message: resp.statusText }; }
      const err = new Error(errBody.message || ` + "`HTTP ${resp.status}`" + `) as Error & { status: number; data: any };
      err.status = resp.status;
      err.data = errBody;
      throw err;
    }

    if (resp.status === 204) {
      return undefined as unknown as T;
    }

    return resp.json();
  }
}
`)

	outputFile := filepath.Join(clientDir, "bffx.gen.ts")
	if err := os.WriteFile(outputFile, []byte(sb.String()), 0o644); err != nil {
		return err
	}

	// Create index.ts entry point
	indexFile := filepath.Join(clientDir, "index.ts")
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		indexContent := "export * from './bffx.gen';\n"
		_ = os.WriteFile(indexFile, []byte(indexContent), 0o644)
	}

	// Create package.json for clients/typescript
	pkgJsonFile := filepath.Join(root, "clients", "typescript", "package.json")
	if _, err := os.Stat(pkgJsonFile); os.IsNotExist(err) {
		pkgJson := fmt.Sprintf(`{
  "name": "@bffx/client",
  "version": "%s",
  "description": "Official TypeScript/JavaScript Client SDK for BFFX",
  "main": "dist/index.js",
  "types": "dist/index.d.ts",
  "scripts": {
    "build": "tsc"
  },
  "devDependencies": {
    "typescript": "^5.4.0"
  }
}
`, "1.0.0")
		_ = os.WriteFile(pkgJsonFile, []byte(pkgJson), 0o644)
	}

	return nil
}
