package generator

import (
	"fmt"
	"os"
	"path/filepath"
)

func GenerateClientIOS(root, name string) error {
	iosDir := filepath.Join(root, "ios")
	if err := os.MkdirAll(iosDir, 0o755); err != nil {
		return err
	}

	// 1. BFFXSDK.swift
	sdkContent := `import Foundation
import Combine

class BFFXClient: ObservableObject {
    static let shared = BFFXClient()
    
    var baseURL = "http://localhost:8080/api/v1"
    var authToken: String? = nil
    
    func setAuth(token: String) {
        self.authToken = token
    }
    
    func request<T: Codable>(_ path: String, method: String = "GET", body: [String: Any]? = nil) async throws -> T {
        guard let url = URL(string: baseURL + path) else {
            throw URLError(.badURL)
        }
        
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        
        if let token = authToken {
            request.addValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        
        if let body = body {
            request.httpBody = try JSONSerialization.data(withJSONObject: body)
        }
        
        let (data, response) = try await URLSession.shared.data(for: request)
        
        guard let httpResponse = response as? HTTPURLResponse, (200...299).contains(httpResponse.statusCode) else {
            throw URLError(.badServerResponse)
        }
        
        return try JSONDecoder().decode(T.self, from: data)
    }
    
    func stream(path: String) -> AsyncStream<String> {
        return AsyncStream { continuation in
            guard let url = URL(string: baseURL + path) else {
                continuation.finish()
                return
            }
            
            var request = URLRequest(url: url)
            request.timeoutInterval = 3600
            
            let task = URLSession.shared.dataTask(with: request) { data, response, error in
                // Extremely simplified SSE bridge for alpha
                if let data = data, let str = String(data: data, encoding: .utf8) {
                    continuation.yield(str)
                }
            }
            task.resume()
        }
    }
}
`
	os.WriteFile(filepath.Join(iosDir, "BFFXClient.swift"), []byte(sdkContent), 0o644)

	// 2. ContentView.swift
	contentView := `import SwiftUI

struct ResourceItem: Codable, Identifiable {
    let id: String
    let name: String
}

struct ContentView: View {
    @State private var items: [ResourceItem] = []
    
    var body: some View {
        NavigationView {
            List(items) { item in
                VStack(alignment: .leading) {
                    Text(item.name)
                        .font(.headline)
                    Text("ID: \(item.id)")
                        .font(.caption)
                        .foregroundColor(.gray)
                }
            }
            .navigationTitle("BFFX App")
            .onAppear {
                Task {
                    // Example load for a generic resource
                    // self.items = try? await BFFXClient.shared.request("/resources/list")
                }
            }
        }
    }
}
`
	os.WriteFile(filepath.Join(iosDir, "ContentView.swift"), []byte(contentView), 0o644)

	fmt.Printf("Generated iOS client scaffolding at %s\n", iosDir)
	return nil
}
