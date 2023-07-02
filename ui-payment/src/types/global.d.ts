// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Dictionary = {[key: string]: any};

type RequireField<T, K extends keyof T> = T & Required<Pick<T, K>>;
