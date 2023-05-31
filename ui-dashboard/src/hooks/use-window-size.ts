import * as React from "react";

interface Size {
    width: number | undefined;
    height: number | undefined;
}

const useWindowSize = (): Size => {
    const [windowSize, setWindowSize] = React.useState<Size>({
        width: undefined,
        height: undefined
    });

    React.useEffect(() => {
        function handleResize() {
            setWindowSize({
                width: window.innerWidth,
                height: window.innerHeight
            });
        }

        window.addEventListener("resize", handleResize);

        handleResize();

        return () => window.removeEventListener("resize", handleResize);
    }, []);

    return windowSize;
};

export default useWindowSize;
