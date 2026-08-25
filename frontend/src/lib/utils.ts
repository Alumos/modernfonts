import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

/**
 * 合并 Tailwind className，并解决冲突的工具类。
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
