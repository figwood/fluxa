"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, Plus, Save, Trash2 } from "lucide-react";
import dagre from "@dagrejs/dagre";
import ReactFlow, {
  addEdge,
  applyEdgeChanges,
  applyNodeChanges,
  Connection,
  ConnectionMode,
  Controls,
  Edge,
  EdgeChange,
  MarkerType,
  Node,
  NodeChange,
  updateEdge
} from "reactflow";
import { apiFetch, putJSON } from "@/lib/api";
import type { Project, WorkflowConfig, WorkflowKind, WorkflowStageConfig, WorkflowStatusConfig, WorkflowTransitionConfig } from "@/lib/types";
import { Field } from "@/components/Field";
import { Alert } from "@/components/ui/alert";
import { Badge, type BadgeProps } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";

const categoryOptions = [
  { label: "normal", value: "normal" },
  { label: "active", value: "active" },
  { label: "warning", value: "warning" },
  { label: "success", value: "success" },
  { label: "terminal", value: "terminal" }
];

const nodeWidth = 150;
const nodeHeight = 50;

function categoryVariant(category: string): BadgeProps["variant"] {
  if (category === "success") return "success";
  if (category === "warning") return "warning";
  if (category === "active") return "info";
  if (category === "terminal") return "secondary";
  return "outline";
}

function nextStatusID(statuses: WorkflowStatusConfig[]) {
  return `status_${statuses.length + 1}`;
}

function nextStageID(stages: WorkflowStageConfig[]) {
  return `stage_${stages.length + 1}`;
}

function nextTransitionID(transitions: WorkflowTransitionConfig[]) {
  return `transition_${transitions.length + 1}`;
}

function sortedStatuses(statuses: WorkflowStatusConfig[]) {
  return [...statuses].sort((a, b) => (a.order_index || 0) - (b.order_index || 0) || a.id.localeCompare(b.id));
}

export function WorkflowEditorView({ projectId = "", workflowKind }: { projectId?: string; workflowKind?: WorkflowKind }) {
  const params = useParams<{ kind: WorkflowKind }>();
  const kind = workflowKind || params.kind;
  const [workflow, setWorkflow] = useState<WorkflowConfig | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [canEdit, setCanEdit] = useState(false);
  const [selectedStatusID, setSelectedStatusID] = useState("");
  const [selectedTransitionID, setSelectedTransitionID] = useState("");
  const [flowNodes, setFlowNodes] = useState<Node[]>([]);
  const [flowEdges, setFlowEdges] = useState<Edge[]>([]);

  const statusOptions = useMemo(
    () => (workflow?.statuses || []).map((status) => ({ label: `${status.name} (${status.id})`, value: status.id })),
    [workflow?.statuses]
  );
  const stageOptions = useMemo(
    () => (workflow?.stages || []).map((stage) => ({ label: `${stage.name} (${stage.id})`, value: stage.id })),
    [workflow?.stages]
  );
  const selectedStatus = useMemo(
    () => (workflow?.statuses || []).find((status) => status.id === selectedStatusID) || null,
    [workflow?.statuses, selectedStatusID]
  );
  const selectedStatusIndex = useMemo(
    () => (workflow?.statuses || []).findIndex((status) => status.id === selectedStatusID),
    [workflow?.statuses, selectedStatusID]
  );
  const selectedTransition = useMemo(
    () => (workflow?.transitions || []).find((transition) => transition.id === selectedTransitionID) || null,
    [workflow?.transitions, selectedTransitionID]
  );
  const selectedTransitionIndex = useMemo(
    () => (workflow?.transitions || []).findIndex((transition) => transition.id === selectedTransitionID),
    [workflow?.transitions, selectedTransitionID]
  );

  const getLayoutedElements = useCallback((nodes: Node[], edges: Edge[]) => {
    const dagreGraph = new dagre.graphlib.Graph();
    dagreGraph.setDefaultEdgeLabel(() => ({}));
    dagreGraph.setGraph({ rankdir: "TB", nodesep: 50, ranksep: 80 });

    nodes.forEach((node) => {
      dagreGraph.setNode(node.id, { width: nodeWidth, height: nodeHeight });
    });
    edges.forEach((edge) => {
      dagreGraph.setEdge(edge.source, edge.target);
    });

    dagre.layout(dagreGraph);

    return {
      nodes: nodes.map((node) => {
        const next = dagreGraph.node(node.id);
        return {
          ...node,
          position: {
            x: next.x - nodeWidth / 2,
            y: next.y - nodeHeight / 2
          }
        };
      }),
      edges
    };
  }, []);

  const getStatusNodeStyle = useCallback((status: WorkflowStatusConfig, selected = false): React.CSSProperties => {
    const base: React.CSSProperties = {
      borderRadius: 5,
      padding: 10,
      background: "#fff",
      border: selected ? "3px solid #87CEEB" : "1px solid #000",
      width: nodeWidth
    };
    if (status.category === "warning") base.background = "#fffbea";
    if (status.category === "success") base.background = "#f0fdf4";
    if (status.category === "active") base.background = "#eff6ff";
    if (status.category === "terminal") base.background = "#f8fafc";
    return base;
  }, []);

  useEffect(() => {
    if (!workflow) return;
    const nodes: Node[] = sortedStatuses(workflow.statuses).map((status) => ({
      id: status.id,
      data: { label: status.name, originalRecord: status },
      position: { x: 0, y: 0 },
      deletable: canEdit && workflow.statuses.length > 1,
      style: getStatusNodeStyle(status)
    }));

    const statusIDs = new Set(workflow.statuses.map((status) => status.id));
    const edges: Edge[] = workflow.transitions
      .filter((transition) => statusIDs.has(transition.from_status) && statusIDs.has(transition.to_status))
      .map((transition) => ({
        id: transition.id,
        source: transition.from_status,
        target: transition.to_status,
        label: transition.name,
        type: "smoothstep",
        markerEnd: { type: MarkerType.ArrowClosed },
        data: { originalRecord: transition },
        deletable: canEdit,
        style: undefined
      }));

    const layouted = getLayoutedElements(nodes, edges);
    setFlowNodes(layouted.nodes);
    setFlowEdges(layouted.edges);
  }, [workflow?.statuses, workflow?.transitions, canEdit, getLayoutedElements, getStatusNodeStyle]);

  useEffect(() => {
    if (!workflow) return;
    const statusByID = new Map(workflow.statuses.map((status) => [status.id, status]));
    setFlowNodes((nodes) => nodes.map((node) => {
      const status = statusByID.get(node.id);
      return {
        ...node,
        selected: selectedStatusID === node.id,
        style: status ? getStatusNodeStyle(status, selectedStatusID === node.id) : node.style
      };
    }));
    setFlowEdges((edges) => edges.map((edge) => ({
      ...edge,
      selected: selectedTransitionID === edge.id,
      style: selectedTransitionID === edge.id ? { stroke: "#87CEEB", strokeWidth: 3 } : undefined
    })));
  }, [workflow, selectedStatusID, selectedTransitionID, getStatusNodeStyle]);

  const load = async () => {
    setLoading(true);
    setError("");
    setMessage("");
    try {
      const [item, project] = await Promise.all([
        apiFetch<WorkflowConfig>(`/projects/${projectId}/workflows/${kind}`),
        apiFetch<Project>(`/projects/${projectId}`)
      ]);
      setWorkflow({ ...item, stages: item.stages || [] });
      setCanEdit(project.current_user_role === "owner" || project.current_user_role === "lead");
    } catch (err: any) {
      setError(err.message || "加载失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (kind) load();
  }, [kind]);

  const updateWorkflow = (patch: Partial<WorkflowConfig>) => {
    if (!workflow) return;
    setWorkflow({ ...workflow, ...patch });
  };

  const updateStage = (index: number, patch: Partial<WorkflowStageConfig>) => {
    if (!workflow) return;
    const previousID = workflow.stages[index]?.id;
    const stages = workflow.stages.map((item, i) => (i === index ? { ...item, ...patch } : item));
    const nextID = patch.id;
    if (nextID && previousID && nextID !== previousID) {
      setWorkflow({
        ...workflow,
        stages,
        statuses: workflow.statuses.map((status) => ({
          ...status,
          stage_id: status.stage_id === previousID ? nextID : status.stage_id
        }))
      });
      return;
    }
    setWorkflow({ ...workflow, stages });
  };

  const updateStatus = (index: number, patch: Partial<WorkflowStatusConfig>) => {
    if (!workflow) return;
    const previousID = workflow.statuses[index]?.id;
    const statuses = workflow.statuses.map((item, i) => (i === index ? { ...item, ...patch } : item));
    const nextID = patch.id;
    if (nextID && previousID && nextID !== previousID) {
      setWorkflow({
        ...workflow,
        initial_status: workflow.initial_status === previousID ? nextID : workflow.initial_status,
        statuses,
        transitions: workflow.transitions.map((transition) => ({
          ...transition,
          from_status: transition.from_status === previousID ? nextID : transition.from_status,
          to_status: transition.to_status === previousID ? nextID : transition.to_status
        }))
      });
      return;
    }
    setWorkflow({ ...workflow, statuses });
  };

  const updateTransition = (index: number, patch: Partial<WorkflowTransitionConfig>) => {
    if (!workflow) return;
    const transitions = workflow.transitions.map((item, i) => (i === index ? { ...item, ...patch } : item));
    setWorkflow({ ...workflow, transitions });
  };

  const onNodesChange = useCallback((changes: NodeChange[]) => {
    setFlowNodes((nodes) => applyNodeChanges(changes, nodes));
  }, []);

  const onEdgesChange = useCallback((changes: EdgeChange[]) => {
    setFlowEdges((edges) => applyEdgeChanges(changes, edges));
  }, []);

  const onNodeClick = useCallback((event: React.MouseEvent, node: Node) => {
    setSelectedStatusID(node.id);
    setSelectedTransitionID("");
  }, []);

  const onEdgeClick = useCallback((event: React.MouseEvent, edge: Edge) => {
    setSelectedTransitionID(edge.id);
    setSelectedStatusID("");
  }, []);

  const onPaneClick = useCallback(() => {
    setSelectedStatusID("");
    setSelectedTransitionID("");
  }, []);

  const onConnect = useCallback((connection: Connection) => {
    const source = connection.source;
    const target = connection.target;
    if (!workflow || !source || !target) return;
    const transition: WorkflowTransitionConfig = {
      id: nextTransitionID(workflow.transitions),
      name: "新流转",
      from_status: source,
      to_status: target,
      action: "",
      order_index: workflow.transitions.length + 1
    };
    setWorkflow({ ...workflow, transitions: [...workflow.transitions, transition] });
    setSelectedTransitionID(transition.id);
    setSelectedStatusID("");
    setFlowEdges((edges) => addEdge({
      id: transition.id,
      source,
      target,
      label: transition.name,
      type: "smoothstep",
      markerEnd: { type: MarkerType.ArrowClosed },
      data: { originalRecord: transition }
    }, edges));
  }, [workflow]);

  const addStatus = () => {
    if (!workflow) return;
    const status: WorkflowStatusConfig = {
      id: nextStatusID(workflow.statuses),
      name: "新状态",
      stage_id: workflow.stages[0]?.id || "",
      category: "normal",
      order_index: workflow.statuses.length + 1
    };
    setWorkflow({ ...workflow, statuses: [...workflow.statuses, status] });
    setSelectedStatusID(status.id);
    setSelectedTransitionID("");
  };

  const addStage = () => {
    if (!workflow) return;
    const stage: WorkflowStageConfig = {
      id: nextStageID(workflow.stages),
      name: "新阶段",
      order_index: workflow.stages.length + 1
    };
    setWorkflow({ ...workflow, stages: [...workflow.stages, stage] });
  };

  const removeStage = (stageID: string) => {
    if (!workflow || workflow.stages.length <= 1) return;
    const stages = workflow.stages.filter((item) => item.id !== stageID);
    const fallbackStageID = stages[0]?.id || "";
    setWorkflow({
      ...workflow,
      stages,
      statuses: workflow.statuses.map((status) => ({
        ...status,
        stage_id: status.stage_id === stageID ? fallbackStageID : status.stage_id
      }))
    });
  };

  const removeStatus = (statusID: string) => {
    if (!workflow) return;
    const statuses = workflow.statuses.filter((item) => item.id !== statusID);
    const transitions = workflow.transitions.filter((item) => item.from_status !== statusID && item.to_status !== statusID);
    const initialStatus = workflow.initial_status === statusID ? (statuses[0]?.id || "") : workflow.initial_status;
    setWorkflow({ ...workflow, statuses, transitions, initial_status: initialStatus });
    setSelectedStatusID("");
    setSelectedTransitionID("");
  };

  const removeTransition = (index: number) => {
    if (!workflow) return;
    setWorkflow({ ...workflow, transitions: workflow.transitions.filter((_, i) => i !== index) });
    if (workflow.transitions[index]?.id === selectedTransitionID) setSelectedTransitionID("");
  };

  const onNodesDelete = useCallback((nodes: Node[]) => {
    if (!canEdit) return;
    const statusIDs = nodes.map((node) => node.id);
    setWorkflow((current) => {
      if (!current || current.statuses.length <= statusIDs.length) return current;
      const deleteIDs = new Set(statusIDs);
      const statuses = current.statuses.filter((status) => !deleteIDs.has(status.id));
      const transitions = current.transitions.filter((transition) => !deleteIDs.has(transition.from_status) && !deleteIDs.has(transition.to_status));
      const initialStatus = deleteIDs.has(current.initial_status) ? (statuses[0]?.id || "") : current.initial_status;
      return { ...current, statuses, transitions, initial_status: initialStatus };
    });
    setSelectedStatusID("");
    setSelectedTransitionID("");
  }, [canEdit]);

  const onEdgesDelete = useCallback((edges: Edge[]) => {
    if (!canEdit) return;
    const transitionIDs = new Set(edges.map((edge) => edge.id));
    setWorkflow((current) => current
      ? { ...current, transitions: current.transitions.filter((transition) => !transitionIDs.has(transition.id)) }
      : current
    );
    setSelectedTransitionID((current) => transitionIDs.has(current) ? "" : current);
  }, [canEdit]);

  const validate = () => {
    if (!workflow) return "工作流未加载";
    if (!workflow.name.trim() || !workflow.initial_status || workflow.stages.length === 0 || workflow.statuses.length === 0) return "请补全名称、阶段、起始状态和状态";
    const stageIDs = new Set<string>();
    for (const stage of workflow.stages) {
      if (!stage.id.trim() || !stage.name.trim()) return "阶段 ID 和名称不能为空";
      if (stageIDs.has(stage.id)) return `阶段 ID 重复：${stage.id}`;
      stageIDs.add(stage.id);
    }
    const statusIDs = new Set<string>();
    for (const status of workflow.statuses) {
      if (!status.id.trim() || !status.name.trim() || !status.stage_id) return "状态 ID、名称和所属阶段不能为空";
      if (statusIDs.has(status.id)) return `状态 ID 重复：${status.id}`;
      if (!stageIDs.has(status.stage_id)) return `状态引用了不存在的阶段：${status.id}`;
      statusIDs.add(status.id);
    }
    if (!statusIDs.has(workflow.initial_status)) return "起始状态不存在";
    const transitionIDs = new Set<string>();
    for (const transition of workflow.transitions) {
      if (!transition.id.trim() || !transition.name.trim() || !transition.from_status || !transition.to_status) return "流转 ID、名称、来源和目标不能为空";
      if (transitionIDs.has(transition.id)) return `流转 ID 重复：${transition.id}`;
      if (!statusIDs.has(transition.from_status) || !statusIDs.has(transition.to_status)) return `流转引用了不存在的状态：${transition.id}`;
      transitionIDs.add(transition.id);
    }
    return "";
  };

  const saveWorkflow = useCallback(async (targetWorkflow: WorkflowConfig) => {
    setError("");
    setMessage("");
    const validationError = (() => {
      if (!targetWorkflow.name.trim() || !targetWorkflow.initial_status || targetWorkflow.stages.length === 0 || targetWorkflow.statuses.length === 0) return "请补全名称、阶段、起始状态和状态";
      const stageIDs = new Set<string>();
      for (const stage of targetWorkflow.stages) {
        if (!stage.id.trim() || !stage.name.trim()) return "阶段 ID 和名称不能为空";
        if (stageIDs.has(stage.id)) return `阶段 ID 重复：${stage.id}`;
        stageIDs.add(stage.id);
      }
      const statusIDs = new Set<string>();
      for (const status of targetWorkflow.statuses) {
        if (!status.id.trim() || !status.name.trim() || !status.stage_id) return "状态 ID、名称和所属阶段不能为空";
        if (statusIDs.has(status.id)) return `状态 ID 重复：${status.id}`;
        if (!stageIDs.has(status.stage_id)) return `状态引用了不存在的阶段：${status.id}`;
        statusIDs.add(status.id);
      }
      if (!statusIDs.has(targetWorkflow.initial_status)) return "起始状态不存在";
      const transitionIDs = new Set<string>();
      for (const transition of targetWorkflow.transitions) {
        if (!transition.id.trim() || !transition.name.trim() || !transition.from_status || !transition.to_status) return "流转 ID、名称、来源和目标不能为空";
        if (transitionIDs.has(transition.id)) return `流转 ID 重复：${transition.id}`;
        if (!statusIDs.has(transition.from_status) || !statusIDs.has(transition.to_status)) return `流转引用了不存在的状态：${transition.id}`;
        transitionIDs.add(transition.id);
      }
      return "";
    })();
    if (validationError) {
      setError(validationError);
      return;
    }
    setSaving(true);
    try {
      const saved = await putJSON<WorkflowConfig>(`/projects/${projectId}/workflows/${kind}`, {
        name: targetWorkflow.name,
        description: targetWorkflow.description,
        initial_status: targetWorkflow.initial_status,
        stages: targetWorkflow.stages,
        statuses: targetWorkflow.statuses,
        transitions: targetWorkflow.transitions
      });
      setWorkflow(saved);
      setSelectedStatusID((current) => saved.statuses.some((status) => status.id === current) ? current : "");
      setSelectedTransitionID((current) => saved.transitions.some((transition) => transition.id === current) ? current : "");
      setMessage("已保存");
    } catch (err: any) {
      setError(err.message || "保存失败");
    } finally {
      setSaving(false);
    }
  }, [kind, projectId]);

  const save = async () => {
    if (!workflow) return;
    const validationError = validate();
    if (validationError) {
      setError(validationError);
      return;
    }
    await saveWorkflow(workflow);
  };

  const onEdgeUpdate = useCallback((oldEdge: Edge, newConnection: Connection) => {
    if (!newConnection.source || !newConnection.target) return;
    setFlowEdges((edges) => updateEdge(oldEdge, newConnection, edges));
    setWorkflow((current) => {
      if (!current) return current;
      const nextWorkflow = {
        ...current,
        transitions: current.transitions.map((transition) => transition.id === oldEdge.id
          ? { ...transition, from_status: newConnection.source || transition.from_status, to_status: newConnection.target || transition.to_status }
          : transition
        )
      };
      void saveWorkflow(nextWorkflow);
      return nextWorkflow;
    });
    setSelectedTransitionID(oldEdge.id);
    setSelectedStatusID("");
  }, [saveWorkflow]);

  if (loading && !workflow) return <div>加载中...</div>;
  if (!workflow) return <Alert>{error || "工作流不存在"}</Alert>;

  return (
    <div className="grid gap-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">{workflow.name}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{workflow.kind === "task" ? "任务工作流" : "发布工作流"}</p>
        </div>
        <div className="flex flex-wrap justify-end gap-2">
          <Link href={`/projects/${projectId}/workflows`}>
            <Button variant="outline">
              <ArrowLeft className="h-4 w-4" />
              返回
            </Button>
          </Link>
        </div>
      </div>

      {error ? <Alert>{error}</Alert> : null}
      {message ? <Alert className="border-emerald-200 bg-emerald-50 text-emerald-700">{message}</Alert> : null}
      {!canEdit ? <Alert>当前角色可查看工作流，但只有项目 Owner、Lead 或管理员可以修改。</Alert> : null}

      <fieldset disabled={!canEdit} className="contents">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between border-b">
          <div>
            <CardTitle>流程图</CardTitle>
            <p className="mt-1 text-sm text-muted-foreground">按状态和流转关系展示；拖动节点调整位置，点选节点或连线后在右侧修改。</p>
          </div>
          <div className="flex gap-2">
            <Button size="sm" variant="outline" onClick={addStatus}>
              <Plus className="h-4 w-4" />
              状态
            </Button>
          </div>
        </CardHeader>
        <CardContent className="grid gap-0 p-0 lg:grid-cols-[minmax(0,1fr)_300px]">
          <div className="h-[600px] bg-[#f8fafc]">
            <ReactFlow
              nodes={flowNodes}
              edges={flowEdges}
              fitView
              onNodeClick={onNodeClick}
              onEdgeClick={onEdgeClick}
              onPaneClick={onPaneClick}
              onConnect={onConnect}
              onEdgeUpdate={onEdgeUpdate}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              onNodesDelete={onNodesDelete}
              onEdgesDelete={onEdgesDelete}
              connectionMode={ConnectionMode.Loose}
              nodesDraggable={canEdit}
              nodesConnectable={canEdit}
              edgesUpdatable={canEdit}
              deleteKeyCode={canEdit ? ["Delete", "Backspace"] : null}
              elementsSelectable
              proOptions={{ hideAttribution: true }}
            >
              <Controls />
            </ReactFlow>
          </div>
          <div className="h-[600px] overflow-y-auto border-l bg-white p-4">
            {selectedStatus && selectedStatusIndex >= 0 ? (
              <div className="grid gap-3">
                <div className="border-b pb-3">
                  <div className="text-sm font-semibold">状态</div>
                  <div className="mt-1 font-mono text-xs text-muted-foreground">{selectedStatus.id}</div>
                </div>
                <Field label="名称">
                  <Input value={selectedStatus.name} onChange={(event) => updateStatus(selectedStatusIndex, { name: event.target.value })} />
                </Field>
                <Field label="类别">
                  <Select value={selectedStatus.category} onChange={(event) => updateStatus(selectedStatusIndex, { category: event.target.value })} options={categoryOptions} />
                </Field>
                <Button type="button" onClick={save} disabled={saving}>
                  <Save className="h-4 w-4" />
                  {saving ? "保存中" : "保存"}
                </Button>
                <Button type="button" variant="destructive" onClick={() => removeStatus(selectedStatus.id)} disabled={workflow.statuses.length <= 1}>
                  <Trash2 className="h-4 w-4" />
                  删除状态
                </Button>
              </div>
            ) : selectedTransition && selectedTransitionIndex >= 0 ? (
              <div className="grid gap-3">
                <div className="border-b pb-3">
                  <div className="text-sm font-semibold">流转</div>
                  <div className="mt-1 font-mono text-xs text-muted-foreground">{selectedTransition.id}</div>
                </div>
                <Field label="名称">
                  <Input value={selectedTransition.name} onChange={(event) => updateTransition(selectedTransitionIndex, { name: event.target.value })} />
                </Field>
                <Field label="来源">
                  <Select value={selectedTransition.from_status} onChange={(event) => updateTransition(selectedTransitionIndex, { from_status: event.target.value })} options={statusOptions} />
                </Field>
                <Field label="目标">
                  <Select value={selectedTransition.to_status} onChange={(event) => updateTransition(selectedTransitionIndex, { to_status: event.target.value })} options={statusOptions} />
                </Field>
                <Field label="动作">
                  <Input value={selectedTransition.action} onChange={(event) => updateTransition(selectedTransitionIndex, { action: event.target.value })} />
                </Field>
                <Button type="button" onClick={save} disabled={saving}>
                  <Save className="h-4 w-4" />
                  {saving ? "保存中" : "保存"}
                </Button>
                <Button type="button" variant="destructive" onClick={() => removeTransition(selectedTransitionIndex)}>
                  <Trash2 className="h-4 w-4" />
                  删除流转
                </Button>
              </div>
            ) : (
              <div className="grid gap-3 text-sm text-muted-foreground">
                <div className="font-medium text-foreground">未选择元素</div>
                <div>点击节点或连线进行编辑；拖动节点可调整画布位置和顺序。</div>
                <Button type="button" onClick={save} disabled={saving}>
                  <Save className="h-4 w-4" />
                  {saving ? "保存中" : "保存"}
                </Button>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>基本信息</CardTitle></CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <Field label="名称">
            <Input value={workflow.name} onChange={(event) => updateWorkflow({ name: event.target.value })} />
          </Field>
          <Field label="起始状态">
            <Select value={workflow.initial_status} onChange={(event) => updateWorkflow({ initial_status: event.target.value })} options={statusOptions} />
          </Field>
          <div className="md:col-span-2">
            <Field label="描述">
              <Textarea value={workflow.description} onChange={(event) => updateWorkflow({ description: event.target.value })} />
            </Field>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>阶段</CardTitle>
          <Button size="sm" variant="outline" onClick={addStage}>
            <Plus className="h-4 w-4" />
            新增阶段
          </Button>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="min-w-44">阶段 ID</TableHead>
                <TableHead className="min-w-44">名称</TableHead>
                <TableHead className="w-28">排序</TableHead>
                <TableHead className="w-20">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {workflow.stages.map((stage, index) => (
                <TableRow key={`${stage.id}-${index}`}>
                  <TableCell>
                    <Input value={stage.id} onChange={(event) => updateStage(index, { id: event.target.value.trim() })} />
                  </TableCell>
                  <TableCell>
                    <Input value={stage.name} onChange={(event) => updateStage(index, { name: event.target.value })} />
                  </TableCell>
                  <TableCell>
                    <Input type="number" value={stage.order_index} onChange={(event) => updateStage(index, { order_index: Number(event.target.value) || 0 })} />
                  </TableCell>
                  <TableCell>
                    <Button type="button" size="sm" variant="ghost" onClick={() => removeStage(stage.id)} disabled={workflow.stages.length <= 1}>
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>状态节点</CardTitle>
          <Button size="sm" variant="outline" onClick={addStatus}>
            <Plus className="h-4 w-4" />
            新增状态
          </Button>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="min-w-44">状态 ID</TableHead>
                <TableHead className="min-w-44">名称</TableHead>
                <TableHead className="min-w-44">所属阶段</TableHead>
                <TableHead className="w-40">类别</TableHead>
                <TableHead className="w-28">标记</TableHead>
                <TableHead className="w-20">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {workflow.statuses.map((status, index) => (
                <TableRow key={`${status.id}-${index}`}>
                  <TableCell>
                    <Input value={status.id} onChange={(event) => updateStatus(index, { id: event.target.value.trim() })} />
                  </TableCell>
                  <TableCell>
                    <Input value={status.name} onChange={(event) => updateStatus(index, { name: event.target.value })} />
                  </TableCell>
                  <TableCell>
                    <Select value={status.stage_id} onChange={(event) => updateStatus(index, { stage_id: event.target.value })} options={stageOptions} />
                  </TableCell>
                  <TableCell>
                    <Select value={status.category} onChange={(event) => updateStatus(index, { category: event.target.value })} options={categoryOptions} />
                  </TableCell>
                  <TableCell>
                    <Badge variant={categoryVariant(status.category)}>{status.category || "normal"}</Badge>
                  </TableCell>
                  <TableCell>
                    <Button type="button" size="sm" variant="ghost" onClick={() => removeStatus(status.id)} disabled={workflow.statuses.length <= 1}>
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>流转边</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="min-w-44">流转 ID</TableHead>
                <TableHead className="min-w-44">名称</TableHead>
                <TableHead className="min-w-48">来源</TableHead>
                <TableHead className="min-w-48">目标</TableHead>
                <TableHead className="min-w-40">动作</TableHead>
                <TableHead className="w-28">排序</TableHead>
                <TableHead className="w-20">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {workflow.transitions.length === 0 ? (
                <TableRow><TableCell colSpan={7}>暂无流转</TableCell></TableRow>
              ) : workflow.transitions.map((transition, index) => (
                <TableRow key={`${transition.id}-${index}`}>
                  <TableCell>
                    <Input value={transition.id} onChange={(event) => updateTransition(index, { id: event.target.value.trim() })} />
                  </TableCell>
                  <TableCell>
                    <Input value={transition.name} onChange={(event) => updateTransition(index, { name: event.target.value })} />
                  </TableCell>
                  <TableCell>
                    <Select value={transition.from_status} onChange={(event) => updateTransition(index, { from_status: event.target.value })} options={statusOptions} />
                  </TableCell>
                  <TableCell>
                    <Select value={transition.to_status} onChange={(event) => updateTransition(index, { to_status: event.target.value })} options={statusOptions} />
                  </TableCell>
                  <TableCell>
                    <Input value={transition.action} onChange={(event) => updateTransition(index, { action: event.target.value })} />
                  </TableCell>
                  <TableCell>
                    <Input type="number" value={transition.order_index} onChange={(event) => updateTransition(index, { order_index: Number(event.target.value) || 0 })} />
                  </TableCell>
                  <TableCell>
                    <Button type="button" size="sm" variant="ghost" onClick={() => removeTransition(index)}>
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
      </fieldset>
    </div>
  );
}
