// 路由相关类型扩展
export namespace Route {
    export interface LinksFunction {
        (): Array<{
            rel: string;
            href: string;
            crossOrigin?: string;
        }>;
    }

    export interface ErrorBoundaryProps {
        error: unknown;
    }
}
