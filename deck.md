ReAct pattern
Alterne **raisonnement** (Thought) et **action** (Tool call) à chaque étape ; observe le résultat, itère jusqu'à réponse finale.

Plan-and-execute pattern
Génère un **plan complet** d'étapes en amont, puis exécute séquentiellement, sans replanifier à chaque action (vs ReAct).

Reflection loops
Agent produit une sortie, **réfléchit** verbalement sur ses erreurs/limites, puis **révise** itérativement jusqu'à critère d'arrêt.

Self-critique loops
Sous-étape de reflection : le modèle **évalue sa propre sortie** contre des critères explicites avant de la **raffiner**.

Tool use / function calling
LLM invoque des **fonctions externes** (API, code) avec arguments structurés (JSON), puis intègre le résultat au contexte.

Tool schema design
Définir nom, description et **paramètres typés** (JSON Schema) d'un tool pour guider le LLM vers un appel correct.

Orchestrator-worker pattern
Un agent **orchestrateur** décompose la tâche, délègue des sous-tâches à des **workers** spécialisés, puis **agrège** leurs résultats.

Hierarchical multi-agent systems
Agents organisés en **niveaux** (superviseur/subordonnés) ; coordination top-down, délégation et **agrégation récursive** des résultats.

Debate-style multi-agent setups
Plusieurs agents **confrontent leurs réponses** de façon **contradictoire** ; un juge ou un consensus arbitre pour améliorer la fiabilité.

Agent short-term memory
Stocke les infos de la **session en cours** (fenêtre de contexte active), perdues une fois la tâche terminée.

Agent long-term memory
Stocke des infos **persistantes entre sessions** (préférences, faits passés), souvent via une **base vectorielle** externe.

Episodic memory
Enregistre des **expériences passées spécifiques** (interactions, décisions) pour les rappeler dans des situations futures similaires.

Context window budget management
Gère la **limite de tokens** disponible en **priorisant** quelles infos garder, résumer, ou supprimer du prompt.

Context compression
**Résume ou condense** l'historique/contexte pour réduire son volume en tokens, tout en préservant l'info essentielle.

Retrieval-augmented prompting
Injecte des **documents externes pertinents** (trouvés via recherche/embeddings) directement dans le prompt avant génération.

Agent eval: task success rate
Mesure la **proportion de tâches** menées à terme et **complétées correctement** ; métrique simple (vs évaluation fine de trajectoire).

Agent eval: trajectory evaluation
Juge la **suite complète des actions** intermédiaires (pas seulement le résultat final) : choix d'outils, raisonnement, efficacité du chemin.

LLM-as-judge
Un modèle (souvent plus puissant) **note la qualité** d'une sortie selon une **rubrique** définie, remplaçant l'évaluation humaine.

Guardrails for agents
**Contraintes de sécurité** (filtres entrée/sortie, limites d'actions) empêchant l'agent de sortir du **périmètre autorisé**.

Sandboxing for autonomous agents
Exécute les actions (code, fichiers) dans un **environnement isolé**, pour **limiter l'impact** si erreur ou comportement malveillant.

Human-in-the-loop design
Insère des **points de validation** ou d'intervention humaine avant les actions **critiques ou irréversibles**.

Failure recovery in agent loops
**Détecte les erreurs** d'exécution (timeout, exception), puis **retente ou change de stratégie** sans bloquer toute la boucle.

Agent observability / tracing
**Journalise chaque étape** (appels, raisonnements, latences) pour **déboguer** et auditer le comportement de l'agent.

Cost/latency tradeoffs in agent pipelines
**Arbitrage** entre modèles précis mais chers/lents, et solutions rapides/économiques mais moins **fiables**.

Prompt injection (vecteur d'attaque sur agents)
**Instructions malveillantes** cachées dans un contenu externe (page web, doc) que l'agent traite comme légitimes, **détournant** son comportement.

State management in multi-step agents
Maintient une **mémoire persistante** (variables, historique, résultats intermédiaires) entre les étapes successives d'une tâche longue.

Graph-based agent frameworks (concepts)
Modélise l'agent comme des **nœuds** (étapes/actions) reliés par des **arêtes conditionnelles**, permettant boucles et branchements explicites (vs chaîne linéaire).
