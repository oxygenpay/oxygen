import * as React from "react";
import Icon from "src/components/Icon";

const NotFoundPage: React.FC = () => {
    return (
        <div className="flex items-center justify-center">
            <div className="mt-20 mb-[4.5rem] sm:mt-56">
                <div className="mx-auto h-32 w-32 flex items-center justify-center mb-4 sm:mb-6">
                    <div className="shrink-0 justify-self-center">
                        <Icon name="not-found" className="h-30 w-30" />
                    </div>
                </div>
                <span className="block mx-auto text-[32px] leading-[32px] text-main-green-1 font-medium text-center mb-4">
                    Not Found
                </span>
                <p className="block mx-auto text-center text-card-desc font-medium text-sm max-w-[180px]">
                    The page you are looking for not found
                </p>
            </div>
        </div>
    );
};

export default NotFoundPage;
