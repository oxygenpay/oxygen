import * as React from "react";
import {useMount} from "react-use";

interface Props {
    expiresAt: string;
    minutesCount: number;
    error: boolean;
}

const ProgressCircle: React.FC<Props> = ({expiresAt, minutesCount, error}) => {
    const [availableTime, setAvailableTime] = React.useState<number>(0);
    const circleRadius = 30;
    const MillisecondsInSecond = 1000;
    const SecondsInMinute = 60;
    const circumference = 2 * Math.PI * circleRadius;

    const updateAvailableTime = () => {
        const availableTime = Date.parse(expiresAt) - +new Date();
        setAvailableTime(availableTime);

        if (availableTime > 0) {
            setTimeout(updateAvailableTime, 1000);
        }
    };

    useMount(() => {
        if (!availableTime) {
            updateAvailableTime();
            return;
        }

        setAvailableTime(0);
        setTimeout(updateAvailableTime, 1000);
    });

    React.useEffect(() => {
        setAvailableTime(0);
    }, [error]);

    const addZerosToNumber = (value: number) => {
        if (value <= 0) {
            return "00";
        }

        return ("0" + value).slice(-2);
    };

    const getAvailableTimeString = () => {
        const minutesCount = Math.floor(availableTime / (MillisecondsInSecond * SecondsInMinute)) % SecondsInMinute;
        const secondsCount = Math.floor(availableTime / MillisecondsInSecond) % SecondsInMinute;

        if (minutesCount < 0 && secondsCount < 0) {
            setAvailableTime(0);
        }

        return addZerosToNumber(minutesCount) + ":" + addZerosToNumber(secondsCount);
    };

    const paymentDuration = minutesCount * SecondsInMinute * MillisecondsInSecond;

    return (
        <div className="flex items-center justify-center mb-4">
            <svg className="transform -rotate-90 w-16 h-16">
                <circle
                    cx="32"
                    cy="32"
                    r="30"
                    stroke="currentColor"
                    strokeWidth="4"
                    fill="transparent"
                    className="text-main-green-3"
                />

                <circle
                    cx="32"
                    cy="32"
                    r="30"
                    stroke="currentColor"
                    strokeWidth="4"
                    fill="transparent"
                    strokeDasharray={circumference}
                    strokeDashoffset={circumference - (availableTime / paymentDuration) * circumference}
                    className="text-main-green-1"
                />
            </svg>
            <span className="absolute font-medium text-lg text-main-green-1">{getAvailableTimeString()}</span>
        </div>
    );
};

export default ProgressCircle;
