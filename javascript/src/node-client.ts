import { AgentAPI } from "./client.js";
import { NodeSkillsResource } from "./resources/skills-node.js";

export class NodeAgentAPI extends AgentAPI {
  declare readonly skills: NodeSkillsResource;

  protected override createSkillsResource(): NodeSkillsResource {
    return new NodeSkillsResource(this.http);
  }
}
