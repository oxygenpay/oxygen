import * as React from "react";
import {RouteContext, RouteContextType} from "@ant-design/pro-components";

const useRouteContext = (): RouteContextType => {
    return React.useContext(RouteContext);
};

export default useRouteContext;
