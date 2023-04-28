package v1

#AcornBuild: {
	buildArgs: [string]: #Args
	context:   string | *"."
	acornfile: string | *"Acornfile"
}

#Build: {
	buildArgs: [string]: string
	context:    string | *"."
	dockerfile: string | *""
	target:     string | *""
}

#EnvVars: *[...string] | {[string]: string}

#Sidecar: {
	#ContainerBase
	init: bool | *false
}

#Container: {
	#ContainerBase
	#WorkloadBase
	labels: [string]:      string
	annotations: [string]: string
	scale?: >=0
	sidecars: [string]: #Sidecar
}

#Job: {
	#ContainerBase
	#WorkloadBase
	labels: [string]:      string
	annotations: [string]: string
	schedule: string | *""
	onDelete: bool | *false
	sidecars: [string]: #Sidecar
}

#WorkloadBase: {
	class?: string
}

#Service: *{
	labels: [string]:      string
	annotations: [string]: string
	default:   bool | *false
	external:  string | *""
	address:   string | *""
	ports:     #PortSingle | *[...#Port] | #PortMap
	container: =~#DNSName | *""
	containerLabels: [string]: string
	secrets: [...=~#DNSName]
	data: {...}
} | {
	labels: [string]:      string
	annotations: [string]: string
	default: bool | *false
	generated: {
		job: =~#DNSName
	}
} | {
	labels:                *[...#ScopedLabel] | #ScopedLabelMap
	annotations:           *[...#ScopedLabel] | #ScopedLabelMap
	default:               bool | *false
	image?:                string
	build?:                string | #AcornBuild
	secrets:               string | *[...#AcornSecretBinding]
	links:                 string | *[...#AcornServiceBinding]
	autoUpgrade:           bool | *false
	autoUpgradeInterval:   string | *""
	notifyUpgrade:         bool | *false
	[=~"mem|memory"]:      int | *{[=~#DNSName]: int}
	[=~"env|environment"]: #EnvVars
	serviceArgs: [string]: #Args
}

#ProbeMap: {
	[=~"ready|readiness|liveness|startup"]: string | #ProbeSpec
}

#PortMap: {
	expose:  #PortSingle | *[...#Port]
	publish: #PortSingle | *[...#Port]
}

#ProbeSpec: {
	type: *"readiness" | "liveness" | "startup"
	exec?: {
		command: [...string]
	}
	http?: {
		url: string
		headers: [string]: string
	}
	tcp?: {
		url: string
	}
	initialDelaySeconds: uint32 | *0
	timeoutSeconds:      uint32 | *1
	periodSeconds:       uint32 | *10
	successThreshold:    uint32 | *1
	failureThreshold:    uint32 | *3
}

#Probes: string | #ProbeMap | [...#ProbeSpec] | null

#FileSecretSpec: {
	name:     string
	key:      string
	onChange: *"redeploy" | "noAction"
}

#FileSpec: {
	mode: =~"^[0-7]{3,4}$" | *"0644"
	{
		content: string
	} | {
		secret: #FileSecretSpec
	}
}

#FileContent: {!~"^secret://"} | {=~"^secret://[a-z][-a-z0-9.]*/[a-z][-a-z0-9]*(.onchange=(redeploy|no-action)|.mode=[0-7]{3,4})*$"} | #FileSpec

#ContainerBase: {
	files: [string]:                  #FileContent
	[=~"dirs|directories"]: [string]: #Dir
	// 1 or both of image or build is required
	image?:                         string
	build?:                         string | #Build
	entrypoint:                     string | *[...string]
	[=~"command|cmd"]:              string | *[...string]
	[=~"env|environment"]:          #EnvVars
	[=~"work[dD]ir|working[dD]ir"]: string | *""
	[=~"interactive|tty|stdin"]:    bool | *false
	ports:                          #PortSingle | *[...#Port] | #PortMap
	[=~"probes|probe"]:             #Probes
	[=~"depends[oO]n|depends_on"]:  string | *[...string]
	[=~"mem|memory"]:               int
	permissions: {
		rules: [...#RuleSpec]
		clusterRules: [...#ClusterRuleSpec]
	}
}

#ShortVolumeRef: "^[a-z][-a-z0-9]*$"
#VolumeRef:      "^volume://.+$"
#EphemeralRef:   "^ephemeral://.*$|^$"
#ContextDirRef:  "^\\./.*$"
#SecretRef:      "^secret://[a-z][-a-z0-9]*(.onchange=(redeploy|no-action))?$"

// The below should work but doesn't. So instead we use the log regexp. This seems like a cue bug
// #Dir: #ShortVolumeRef | #VolumeRef | #EphemeralRef | #ContextDirRef | #SecretRef
#Dir: =~"^[a-z][-a-z0-9]*$|^volume://.+$|^ephemeral://.*$|^$|^\\./.*$|^secret://[a-z][-a-z0-9.]*(.onchange=(redeploy|no-action))?$"

#PortSingle: (>0 & <65536) | =~#PortRegexp
#Port:       (>0 & <65536) | =~#PortRegexp | #PortSpec
#PortRegexp: #"^([a-z][-a-z0-9.]+:)?([0-9]+:)?([a-z][-a-z0-9]+:)?([0-9]+)(/(tcp|udp|http))?$"#

#PortSpec: {
	publish:    bool | *false
	port:       int | *targetPort
	targetPort: int | *port
	protocol:   *"" | "tcp" | "udp" | "http"
}

// Allowing [resourceType:][resourceName:][some.random/key]
#ScopedLabelMapKey: =~"^([a-z][-a-z0-9]+:)?([a-z][-a-z0-9]+:)?([a-z][-a-z0-9./]+)?$"
#ScopedLabelMap: {[#ScopedLabelMapKey]: string}
#ScopedLabel: {
	resourceType: =~#DNSName | *""
	resourceName: string | *""
	key:          =~"[a-z][-a-z0-9./][a-z]*"
	value:        string | *""
}

#RuleSpec: {
	verbs: [...string]
	verb?: string
	apiGroups: [...string]
	apiGroup?: string
	resources: [...string]
	resource?: string
	resourceNames: [...string]
	resourceName?: string
	nonResourceURLs: [...string]
	scope?: string
	scopes: [...string]
} | string

#ClusterRuleSpec: {
	verbs: [...string]
	namespaces: [...string]
	apiGroups: [...string]
	resources: [...string]
	resourceNames: [...string]
	nonResourceURLs: [...string]
} | string

#Image: {
	image:           string | *""
	acornBuild?:     string | *#AcornBuild
	containerBuild?: string | *#Build
}

#AccessMode: "readWriteMany" | "readWriteOnce" | "readOnlyMany"

#Volume: {
	external: string | *""
	labels: [string]:      string
	annotations: [string]: string
	class:        string | *""
	size:         int | *"" | string
	accessModes?: [#AccessMode, ...#AccessMode] | #AccessMode
}

#SecretBase: {
	external: string | *""
	labels: [string]:      string
	annotations: [string]: string
}

#SecretOpaque: {
	#SecretBase
	type: "opaque"
	params?: [string]: _
	data: [string]:    string
}

#SecretTemplate: {
	#SecretBase
	type: "template"
	data: [string]: string
}

#SecretToken: {
	#SecretBase
	type: "token"
	params: {
		// The character set used in the generated string
		characters: string | *"bcdfghjklmnpqrstvwxz2456789"
		// The length of the token to be generated
		length: (>=0 & <=256) | *54
	}
	data: {
		token?: string
	}
}

#SecretBasicAuth: {
	#SecretBase
	type: "basic"
	data: {
		username?: string
		password?: string
	}
}

#SecretGenerated: {
	#SecretBase
	type: "generated"
	params: {
		job:    string
		format: *"" | "text" | "json" | "aml"
	}
	data: {}
}

#Secret: *#SecretOpaque | #SecretBasicAuth | #SecretGenerated | #SecretTemplate | #SecretToken

#AcornSecretBinding: {
	secret: string
	target: string
} | string

#AcornServiceBinding: {
	target:  string
	service: string
} | string

#AcornVolumeBinding: {
	target:       string
	class:        string | *""
	size:         int | *"" | string
	accessModes?: [#AccessMode, ...#AccessMode] | #AccessMode
} | string

#AcornPublishPortBinding: {
	port:              int | *targetPort
	hostname:          string | *""
	targetPort:        int | *port
	targetServiceName: =~#DNSName
	protocol:          *"" | "tcp" | "udp" | "http"
} | string | int

#Router: {
	labels: [string]:      string
	annotations: [string]: string
	routes: [...#Route] | #RouteMap
}

#Route: {
	#RouteTarget
	path: =~#PathName
}

#RouteTarget: {
	pathType:          "exact" | *"prefix"
	targetServiceName: =~#DNSName
	targetPort?:       int
}

#RouteMap: [=~#PathName]: {
	=~#RouteTargetName | #RouteTarget
}

#Acorn: {
	labels:                *[...#ScopedLabel] | #ScopedLabelMap
	annotations:           *[...#ScopedLabel] | #ScopedLabelMap
	image?:                string
	build?:                string | #AcornBuild
	publish:               int | string | *[...#AcornPublishPortBinding]
	volumes:               string | *[...#AcornVolumeBinding]
	secrets:               string | *[...#AcornSecretBinding]
	links:                 string | *[...#AcornServiceBinding]
	autoUpgrade:           bool | *false
	autoUpgradeInterval:   string | *""
	notifyUpgrade:         bool | *false
	[=~"mem|memory"]:      int | *{[=~#DNSName]: int}
	[=~"env|environment"]: #EnvVars
	deployArgs: [string]: #Args
	profiles: [...string]
}

#RouteTargetName: "^[a-z][-a-z0-9]*(:[0-9]+)?$"

#PathName: "^/.*$"

#DNSName: "^[a-z][-a-z0-9]*$"

#Args: string | int | float | bool | [...string] | {...}

#App: {
	args: [string]: #Args
	profiles: [string]: [string]: #Args
	[=~"local[dD]ata"]: {...}
	containers: [=~#DNSName]: #Container
	jobs: [=~#DNSName]:       #Job
	images: [=~#DNSName]:     #Image
	volumes: [=~#DNSName]:    #Volume
	secrets: [=~#DNSName]:    #Secret
	acorns: [=~#DNSName]:     #Acorn
	routers: [=~#DNSName]:    #Router
	services: [=~#DNSName]:   #Service
	labels: [string]:         string
	annotations: [string]:    string
}
