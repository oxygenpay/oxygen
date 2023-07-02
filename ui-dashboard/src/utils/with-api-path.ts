import config from "src/config.json";

const withApiPath = (path: string): string => {
    return config.dashboardApi + path;
};

export default withApiPath;
