import { type RouteConfig, route, index, layout, prefix } from "@react-router/dev/routes";

export default [
    // 将首页放入布局中
    layout("./routes/layouts/dashboard-layout.tsx", [
        // 首页
        index("./routes/home.tsx"),

        // 协议管理相关路由
        ...prefix("protocols", [
            // 协议列表已由首页处理

            // 新建协议 - 使用protocols/new.tsx而不是顶层的new.tsx
            route("new", "./routes/protocols/new.tsx"),

            // 协议详情
            route(":protocolId", "./routes/protocols/detail.tsx"),

            // 编辑协议
            route(":protocolId/edit", "./routes/protocols/edit.tsx"),

            // 编辑协议配置 - 使用protocols/edit-config.tsx
            route(":protocolId/edit-config", "./routes/protocols/edit-config.tsx"),
            route(":protocolId/versions/new", "./routes/protocols/versions/new.tsx"),
            // GlobalMap相关路由
            route(":protocolId/globalmaps/new", "./routes/protocols/globalmaps/new.tsx"),
        ]),

        // 独立版本相关路由
        ...prefix("versions", [
            route(":versionId/edit", "./routes/versions/edit.tsx"),
            route(":versionId/orchestration", "./routes/versions/orchestration.tsx"),
        ]),

        // GlobalMap相关独立路由
        ...prefix("globalmaps", [
            route(":globalMapId/edit", "./routes/globalmaps/edit.tsx"),
        ]),

        // 测试相关路由
        ...prefix("test", [
            route("section", "./routes/test/section.tsx"),
        ]),
    ]),
] satisfies RouteConfig;
