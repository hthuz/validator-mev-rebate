package main

import (
	"net/http"
)

// NewRootHandler 创建根路径处理器，显示所有可用端点
func NewRootHandler(api *MevShareAPI) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 根路径 GET 请求返回 HTML 页面
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(indexHTML))
			return
		}

		// POST 请求或其他路径交给 JSON-RPC 处理器
		if r.Method == http.MethodPost {
			NewJSONRPCHandler(api).ServeHTTP(w, r)
			return
		}

		http.Error(w, "Not found", http.StatusNotFound)
	})
}

// indexHTML 根页面 HTML 内容
const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Validator MEV Rebate Node</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #0a0e27;
            color: #e0e0e0;
            line-height: 1.6;
            min-height: 100vh;
        }
        .container { max-width: 1200px; margin: 0 auto; padding: 40px 20px; }
        header { text-align: center; margin-bottom: 50px; }
        h1 {
            font-size: 2.5em;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            margin-bottom: 10px;
        }
        .subtitle { color: #8892b0; font-size: 1.1em; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(350px, 1fr)); gap: 25px; }
        .card {
            background: #161b33;
            border-radius: 12px;
            padding: 25px;
            border: 1px solid #2d3561;
            transition: transform 0.2s, border-color 0.2s;
        }
        .card:hover { transform: translateY(-5px); border-color: #667eea; }
        .card h2 {
            color: #667eea;
            font-size: 1.3em;
            margin-bottom: 20px;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .endpoint {
            background: #0f1429;
            border-radius: 8px;
            padding: 15px;
            margin-bottom: 12px;
            border-left: 3px solid #667eea;
            transition: background 0.2s;
        }
        .endpoint:hover { background: #1a1f3a; }
        .endpoint:last-child { margin-bottom: 0; }
        .method {
            display: inline-block;
            padding: 3px 10px;
            border-radius: 4px;
            font-size: 0.75em;
            font-weight: bold;
            margin-right: 10px;
        }
        .method.get { background: #22c55e; color: #000; }
        .method.post { background: #3b82f6; color: #fff; }
        .path {
            font-family: 'Monaco', 'Menlo', monospace;
            color: #a5b4fc;
            font-size: 0.95em;
            word-break: break-all;
            text-decoration: none;
        }
        .path:hover { text-decoration: underline; color: #fff; }
        .desc {
            color: #8892b0;
            font-size: 0.9em;
            margin-top: 8px;
        }
        .jsonrpc-method {
            background: #1e2746;
            padding: 8px 12px;
            border-radius: 6px;
            margin-bottom: 8px;
            font-family: monospace;
            color: #c084fc;
        }
        footer {
            text-align: center;
            margin-top: 50px;
            color: #64748b;
            font-size: 0.9em;
        }
        a { color: #667eea; text-decoration: none; }
        a:hover { text-decoration: underline; }
        .link-row {
            display: flex;
            align-items: center;
            flex-wrap: wrap;
            gap: 10px;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Validator MEV Rebate Node</h1>
            <p class="subtitle">MEV-Share 兼容的 Validator MEV 返利节点</p>
        </header>

        <div class="grid">
            <div class="card">
                <h2>📡 JSON-RPC 接口</h2>
                <p class="desc" style="margin-bottom: 15px;">所有 JSON-RPC 请求都发送到 <code>POST /</code></p>
                <div class="jsonrpc-method">mev_sendBundle - 提交 Bundle</div>
                <div class="jsonrpc-method">mev_simBundle - 模拟 Bundle</div>
                <div class="jsonrpc-method">eth_cancelBundleByHash - 取消 Bundle</div>
            </div>

            <div class="card">
                <h2>❤️ 健康检查</h2>
                <a href="/health" class="endpoint" style="display: block; text-decoration: none; color: inherit;">
                    <div class="link-row">
                        <span class="method get">GET</span>
                        <span class="path">/health</span>
                    </div>
                    <p class="desc">服务健康状态检查</p>
                </a>
            </div>

            <div class="card">
                <h2>📊 区块指标</h2>
                <a href="/metrics/recent" class="endpoint" style="display: block; text-decoration: none; color: inherit;">
                    <div class="link-row">
                        <span class="method get">GET</span>
                        <span class="path">/metrics/recent</span>
                    </div>
                    <p class="desc">获取最近区块的指标列表</p>
                </a>
                <div class="endpoint">
                    <div class="link-row">
                        <span class="method get">GET</span>
                        <span class="path">/metrics/block/{blockNumber}</span>
                    </div>
                    <p class="desc">获取指定区块的 MEV 指标</p>
                </div>
            </div>

            <div class="card">
                <h2>👤 Validator 指标</h2>
                <a href="/metrics/validators" class="endpoint" style="display: block; text-decoration: none; color: inherit;">
                    <div class="link-row">
                        <span class="method get">GET</span>
                        <span class="path">/metrics/validators</span>
                    </div>
                    <p class="desc">获取所有 Validators 的列表</p>
                </a>
                <div class="endpoint">
                    <div class="link-row">
                        <span class="method get">GET</span>
                        <span class="path">/metrics/validator/{address}</span>
                    </div>
                    <p class="desc">获取指定 Validator 的历史表现</p>
                </div>
            </div>

            <div class="card">
                <h2>🔍 Searcher 指标</h2>
                <a href="/metrics/searchers" class="endpoint" style="display: block; text-decoration: none; color: inherit;">
                    <div class="link-row">
                        <span class="method get">GET</span>
                        <span class="path">/metrics/searchers</span>
                    </div>
                    <p class="desc">获取所有 Searchers 的列表</p>
                </a>
                <div class="endpoint">
                    <div class="link-row">
                        <span class="method get">GET</span>
                        <span class="path">/metrics/searcher/{address}</span>
                    </div>
                    <p class="desc">获取指定 Searcher 的指标</p>
                </div>
            </div>

            <div class="card">
                <h2>🌍 全局统计</h2>
                <a href="/metrics/global" class="endpoint" style="display: block; text-decoration: none; color: inherit;">
                    <div class="link-row">
                        <span class="method get">GET</span>
                        <span class="path">/metrics/global</span>
                    </div>
                    <p class="desc">获取全局 MEV 统计信息</p>
                </a>
            </div>
        </div>

        <footer>
            <p>Validator MEV Rebate Node | <a href="/health">Health Check</a> | <a href="/metrics/global">Global Metrics</a></p>
        </footer>
    </div>
</body>
</html>`
