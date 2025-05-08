// web/server.mjs
import { createRequestHandler } from "@react-router/express";
// import { installGlobals } from "@react-router/node";
import express from "express";
import { createProxyMiddleware } from 'http-proxy-middleware';
import path from 'path';
import { fileURLToPath } from 'url';

// Polyfills for node environment (e.g., fetch)
// installGlobals();

// ESM equivalent of __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const app = express();

// 代理 /api 请求到后端服务
const backendTarget = process.env.BACKEND_INTERNAL_URL || "http://localhost:8081"; // 备选开发时地址
console.log(`[Server] API calls to /api will be proxied to: ${backendTarget}`);

app.use(
    "/api",
    createProxyMiddleware({
        target: backendTarget,
        changeOrigin: true,
        // 可选：如果后端API路径不需要/api前缀，可以重写
        // pathRewrite: { '^/api': '' },
        onError: (err, req, res, target) => {
            console.error('[Proxy Error]', req.method, req.url, err);
            if (res && !res.headersSent) {
                res.writeHead(500, { 'Content-Type': 'application/json' });
            }
            if (res && !res.writableEnded) {
                res.end(JSON.stringify({ message: 'Error connecting to API (proxy error)', error: err.message, target: target }));
            }
        },
        onProxyReq: (proxyReq, req, res) => {
            console.log(`[Proxy Req] Proxied ${req.method} ${req.originalUrl} to ${backendTarget}${proxyReq.path}`);
        },
        onProxyRes: (proxyRes, req, res) => {
            console.log(`[Proxy Res] Received ${proxyRes.statusCode} from ${backendTarget}${req.originalUrl}`);
        }
    })
);

// 提供静态资源 (客户端构建产物)
const clientBuildPath = path.resolve(__dirname, 'build/client');
console.log(`[Server] Serving static files from: ${clientBuildPath}`);
app.use(express.static(clientBuildPath, { index: false })); // index: false, React Router会处理根路由


// React Router 请求处理器应该在代理和静态文件之后，处理所有其他请求
// 动态导入服务器构建产物
const serverBuildPath = path.resolve(__dirname, 'build/server/index.js');
console.log(`[Server] Attempting to load server build from: ${serverBuildPath}`);

(async () => {
    try {
        const build = await import(serverBuildPath);
        app.all("*", createRequestHandler({ build }));
        console.log("[Server] React Router request handler configured with server build.");
    } catch (error) {
        console.error("[Server] Failed to load server build from:", serverBuildPath);
        console.error("[Server] Error details:", error);
        // 如果服务器构建加载失败, 回退到仅客户端渲染模式或显示错误
        // 这通常意味着ssr:true的构建产物没有正确生成或复制
        // 在这种情况下，我们仍然需要让 React Router 处理路由，但它可能只能进行客户端渲染
        // 或者，我们可以只服务一个静态的 index.html (如果存在) 并让客户端接管
        // 但更可能的是，这是一个致命错误，表明构建/部署过程有问题

        // 尝试加载客户端的 index.html 作为回退，但这可能无法正确工作，取决于你的路由设置
        // app.get('*', (req, res) => res.sendFile(path.resolve(clientBuildPath, 'index.html')));

        // 或者显示一个更通用的错误
        app.all("*", (req, res) => {
            res.status(500).send(
                `<h1>Server Error</h1>
         <p>Failed to load server build. Please check server logs for more details.</p>
         <p>Server build path attempted: ${serverBuildPath}</p>
         <p>Error: ${error.message}</p>`
            );
        });
        console.log("[Server] React Router request handler configured with a fallback due to server build load failure.");
    }

    const port = parseInt(process.env.PORT || "3000", 10);
    app.listen(port, "0.0.0.0", () => {
        console.log(`[Server] Express server with API proxy and React Router started on http://0.0.0.0:${port}`);
    });
})();
