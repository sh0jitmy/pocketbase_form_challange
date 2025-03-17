package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

const (
	pbURL          = "http://localhost:8090" // PocketBaseのURL
	targetCollection = "radiospec"               // 連携するコレクション名
)
const pbToken="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJjb2xsZWN0aW9uSWQiOiJwYmNfMzE0MjYzNTgyMyIsImV4cCI6MTc0MjMwNjU5MiwiaWQiOiJmZThnMjVxNjBsMzI5MDMiLCJyZWZyZXNoYWJsZSI6ZmFsc2UsInR5cGUiOiJhdXRoIn0.VagriE2fMwQ-Cfhb3Ls4VvDsayPoZke9oji7d5hmAOo"

var excludeNameList = []string{"id","created","updated"}


func main() {
	http.HandleFunc("/", formHandler)
	http.HandleFunc("/submit", submitHandler)

	fmt.Println("🚀 サービスを http://localhost:48080 で起動中...")
	log.Fatal(http.ListenAndServe(":48080", nil))
}

// フォーム生成ハンドラ
func formHandler(w http.ResponseWriter, r *http.Request) {
	schema, err := fetchSchema(targetCollection)
	if err != nil {
		http.Error(w, "スキーマ取得失敗", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(generateFormHTML(schema)))
}

// PocketBaseからコレクションのスキーマを取得
func fetchSchema(collectionName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/collections/%s", pbURL, collectionName)
	req,_:= http.NewRequest("GET",url,nil)
	req.Header.Set("Authorization",pbToken)

	client := &http.Client{}
	resp,err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PocketBase APIエラー: %s", resp.Status)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}

func excludeNameCheck(name string) bool {
	for _,ex := range excludeNameList {
		if ex == name {
			return true
		}
	}
	return false
}

// フォームHTMLを生成（Tailwind適用）
func generateFormHTML(schema map[string]interface{}) string {
	var sb strings.Builder

	sb.WriteString(`
	<!DOCTYPE html>
	<html lang="ja">
	<head>
		<meta charset="UTF-8">
		<title>PocketBase Form</title>
		<script src="https://cdn.tailwindcss.com"></script>
	</head>
	<body class="bg-gray-100 p-8">
		<div class="max-w-xl mx-auto bg-white p-6 rounded-lg shadow-lg">
			<h1 class="text-2xl font-bold mb-6">データ送信フォーム</h1>
			<form method="POST" action="/submit" class="space-y-4">
	`)

	fields, ok := schema["fields"].([]interface{})
	if !ok {
		return "無効なスキーマ"
	}

	for _, value := range fields {
		column := value.(map[string]interface{})
		name := column["name"].(string)
		fieldType := column["type"].(string)
		
		if excludeNameCheck (name) {
			continue
		}

		sb.WriteString(fmt.Sprintf(`
			<div>
				<label for="%s" class="block text-sm font-medium text-gray-700">%s:</label>`, name, name))

		switch fieldType {
		case "text":
			sb.WriteString(fmt.Sprintf(`
				<input type="text" id="%s" name="%s" class="mt-1 p-2 w-full border rounded-md" required>`, name, name))
		case "email":
			sb.WriteString(fmt.Sprintf(`
				<input type="email" id="%s" name="%s" class="mt-1 p-2 w-full border rounded-md" required>`, name, name))
		case "number":
			sb.WriteString(fmt.Sprintf(`
				<input type="number" id="%s" name="%s" class="mt-1 p-2 w-full border rounded-md">`, name, name))
		case "bool":
			sb.WriteString(fmt.Sprintf(`
				<input type="checkbox" id="%s" name="%s" class="mt-1">`, name, name))
		default:
			sb.WriteString(fmt.Sprintf(`
				<input type="text" id="%s" name="%s" class="mt-1 p-2 w-full border rounded-md">`, name, name))
		}

		sb.WriteString("</div>")
	}

	sb.WriteString(`
				<button type="submit" class="w-full bg-blue-500 text-white p-2 rounded-md hover:bg-blue-600">送信</button>
			</form>
		</div>
	</body>
	</html>`)

	return sb.String()
}

// フォームデータをPocketBaseに送信
func submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "無効なメソッド", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "フォーム解析エラー", http.StatusBadRequest)
		return
	}

	formData := make(map[string]interface{})
	for key, values := range r.Form {
		if len(values) > 0 {
			// チェックボックスはonをboolに変換
			if values[0] == "on" {
				formData[key] = true
			} else {
				formData[key] = values[0]
			}
		}
	}

	payload, err := json.Marshal(formData)
	if err != nil {
		http.Error(w, "データ変換エラー", http.StatusInternalServerError)
		return
	}

	url := fmt.Sprintf("%s/api/collections/%s/records", pbURL, targetCollection)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		http.Error(w, "リクエスト作成エラー", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// 認証が必要な場合はここにトークンをセット
	// req.Header.Set("Authorization", "Bearer YOUR_API_TOKEN")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "PocketBase送信エラー", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Fprintf(w, "✅ データ送信成功: %s", string(body))
}
