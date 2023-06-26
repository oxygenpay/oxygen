import React from "react";
import {useBetween} from "use-between";
import {posthogConfig} from "../providers/posthog";

interface PosthogStatus {
    isPosthogActive: boolean;
    setIsPosthogActive: (isActive: boolean) => void;
}

const usePosthogStatus = (): PosthogStatus => {
    const [isPosthogActive, setIsPosthogActive] = React.useState<boolean>(posthogConfig?.apiKey?.length > 0 ?? false);

    return {
        isPosthogActive,
        setIsPosthogActive
    };
};

const useSharedPosthogStatus = () => useBetween(usePosthogStatus);

export default useSharedPosthogStatus;
