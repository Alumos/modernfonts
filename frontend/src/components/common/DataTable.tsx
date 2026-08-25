import type { ReactNode } from "react"

import { Table, TableBody, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { cn } from "@/lib/utils"

type DataTableHeader = string | { label: string; className?: string; content?: ReactNode }

export function DataTable({ headers, children }: { headers: DataTableHeader[]; children: ReactNode }) {
  return (
    <div className="min-w-0 max-w-full overflow-x-auto border-t">
      <Table className="w-max min-w-full table-auto [&_td]:whitespace-nowrap">
        <TableHeader>
          <TableRow>
            {headers.map((header) => {
              const label = typeof header === "string" ? header : header.label
              const className = typeof header === "string" ? undefined : header.className
              const content = typeof header === "string" ? label : (header.content ?? label)
              return (
                <TableHead className={cn(className)} key={label}>
                  {content}
                </TableHead>
              )
            })}
          </TableRow>
        </TableHeader>
        <TableBody>{children}</TableBody>
      </Table>
    </div>
  )
}
