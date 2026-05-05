"use client";

import { useMemo } from "react";
import { GitBranch } from "lucide-react";
import type { Issue } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../../navigation";
import { StatusIcon } from "./status-icon";
import { buildIssueTree, type IssueTreeNode } from "../utils/issue-tree";

function IssueTreeRow({
  node,
  currentIssueId,
  activePath,
  depth,
}: {
  node: IssueTreeNode;
  currentIssueId: string;
  activePath: Set<string>;
  depth: number;
}) {
  const paths = useWorkspacePaths();
  const issue = node.issue;
  const isCurrent = issue.id === currentIssueId;
  const isInPath = activePath.has(issue.id);

  return (
    <li>
      <AppLink
        href={paths.issueDetail(issue.id)}
        aria-current={isCurrent ? "page" : undefined}
        className={cn(
          "group/tree-row flex min-w-0 items-center gap-1.5 rounded-md px-2 py-1.5 text-xs transition-colors hover:bg-accent/50",
          isCurrent && "bg-accent/70 text-accent-foreground",
          !isCurrent && isInPath && "text-foreground",
          !isCurrent && !isInPath && "text-muted-foreground",
        )}
        style={{ paddingLeft: `${8 + depth * 14}px` }}
      >
        <StatusIcon status={issue.status} className="h-3.5 w-3.5 shrink-0" />
        <span className="shrink-0 tabular-nums font-medium text-muted-foreground">
          {issue.identifier}
        </span>
        <span className="truncate group-hover/tree-row:text-foreground">
          {issue.title}
        </span>
      </AppLink>
      {node.children.length > 0 && (
        <ul className="mt-0.5 border-l border-border/60 ml-[14px] space-y-0.5">
          {node.children.map((child) => (
            <IssueTreeRow
              key={child.issue.id}
              node={child}
              currentIssueId={currentIssueId}
              activePath={activePath}
              depth={depth + 1}
            />
          ))}
        </ul>
      )}
    </li>
  );
}

export function IssueTreeGraph({
  issues,
  currentIssueId,
}: {
  issues: Issue[];
  currentIssueId: string;
}) {
  const tree = useMemo(
    () => buildIssueTree(issues, currentIssueId),
    [issues, currentIssueId],
  );
  const activePath = useMemo(
    () => new Set(tree?.currentPath ?? []),
    [tree],
  );

  if (!tree) return null;

  return (
    <div>
      <div className="mb-2 flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium">
        <span>Graph</span>
        <GitBranch className="ml-auto !size-3 shrink-0 text-muted-foreground" />
      </div>
      <ul className="space-y-0.5 pl-0">
        <IssueTreeRow
          node={tree.root}
          currentIssueId={currentIssueId}
          activePath={activePath}
          depth={0}
        />
      </ul>
    </div>
  );
}
