package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Validator struct {
	filename string
	errors   []string
}

func (v *Validator) errorf(line int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if line > 0 {
		errorMsg := fmt.Sprintf("%s:%d %s", v.filename, line, msg)
		v.errors = append(v.errors, errorMsg)
		fmt.Printf("DEBUG: Added error: %s\n", errorMsg) // Диагностика
	} else {
		errorMsg := fmt.Sprintf("%s %s", v.filename, msg)
		v.errors = append(v.errors, errorMsg)
		fmt.Printf("DEBUG: Added error: %s\n", errorMsg) // Диагностика
	}
}

func (v *Validator) validateTopLevel(doc *yaml.Node) {
	fmt.Printf("DEBUG: Starting validation for document at line %d\n", doc.Line)

	requiredFields := []string{"apiVersion", "kind", "metadata", "spec"}

	fields := make(map[string]*yaml.Node)
	for i := 0; i < len(doc.Content); i += 2 {
		if i+1 < len(doc.Content) {
			key := doc.Content[i]
			value := doc.Content[i+1]
			fields[key.Value] = value
			fmt.Printf("DEBUG: Found field '%s' at line %d\n", key.Value, key.Line)
		}
	}

	for _, field := range requiredFields {
		if node, exists := fields[field]; !exists {
			v.errorf(doc.Line, "%s is required", field)
		} else {
			fmt.Printf("DEBUG: Validating field '%s' at line %d\n", field, node.Line)
			switch field {
			case "apiVersion":
				v.validateString(node, "apiVersion", []string{"v1"})
			case "kind":
				v.validateString(node, "kind", []string{"Pod"})
			case "metadata":
				v.validateMetadata(node)
			case "spec":
				v.validateSpec(node)
			}
		}
	}
}

func (v *Validator) validateMetadata(metadata *yaml.Node) {
	fmt.Printf("DEBUG: Validating metadata at line %d\n", metadata.Line)

	if metadata.Kind != yaml.MappingNode {
		v.errorf(metadata.Line, "metadata must be object")
		return
	}

	fields := make(map[string]*yaml.Node)
	for i := 0; i < len(metadata.Content); i += 2 {
		if i+1 < len(metadata.Content) {
			key := metadata.Content[i]
			value := metadata.Content[i+1]
			fields[key.Value] = value
			fmt.Printf("DEBUG: Metadata field '%s' at line %d\n", key.Value, key.Line)
		}
	}

	if name, exists := fields["name"]; !exists {
		v.errorf(metadata.Line, "name is required")
	} else {
		fmt.Printf("DEBUG: Validating name '%s' at line %d\n", name.Value, name.Line)
		v.validateRequiredString(name, "name")
	}

	if namespace, exists := fields["namespace"]; exists {
		v.validateString(namespace, "namespace", nil)
	}

	if labels, exists := fields["labels"]; exists {
		if labels.Kind != yaml.MappingNode {
			v.errorf(labels.Line, "labels must be object")
		}
	}
}

func (v *Validator) validateSpec(spec *yaml.Node) {
	fmt.Printf("DEBUG: Validating spec at line %d\n", spec.Line)

	if spec.Kind != yaml.MappingNode {
		v.errorf(spec.Line, "spec must be object")
		return
	}

	fields := make(map[string]*yaml.Node)
	for i := 0; i < len(spec.Content); i += 2 {
		if i+1 < len(spec.Content) {
			key := spec.Content[i]
			value := spec.Content[i+1]
			fields[key.Value] = value
			fmt.Printf("DEBUG: Spec field '%s' at line %d\n", key.Value, key.Line)
		}
	}

	if containers, exists := fields["containers"]; !exists {
		v.errorf(spec.Line, "containers is required")
	} else {
		v.validateContainers(containers)
	}

	if os, exists := fields["os"]; exists {
		v.validatePodOS(os)
	}
}

func (v *Validator) validatePodOS(podOS *yaml.Node) {
	fmt.Printf("DEBUG: Validating OS at line %d, value: %s\n", podOS.Line, podOS.Value)

	if podOS.Kind == yaml.ScalarNode {
		// Обработка когда os указан как строка
		v.validateString(podOS, "os", []string{"linux", "windows"})
		return
	}

	if podOS.Kind != yaml.MappingNode {
		v.errorf(podOS.Line, "os must be object")
		return
	}

	fields := make(map[string]*yaml.Node)
	for i := 0; i < len(podOS.Content); i += 2 {
		if i+1 < len(podOS.Content) {
			key := podOS.Content[i]
			value := podOS.Content[i+1]
			fields[key.Value] = value
		}
	}

	if name, exists := fields["name"]; !exists {
		v.errorf(podOS.Line, "os.name is required")
	} else {
		v.validateString(name, "os.name", []string{"linux", "windows"})
	}
}

func (v *Validator) validateContainers(containers *yaml.Node) {
	fmt.Printf("DEBUG: Validating containers at line %d\n", containers.Line)

	if containers.Kind != yaml.SequenceNode {
		v.errorf(containers.Line, "containers must be array")
		return
	}

	containerNames := make(map[string]bool)
	for i, container := range containers.Content {
		fmt.Printf("DEBUG: Validating container %d at line %d\n", i, container.Line)
		v.validateContainer(container, containerNames)
	}
}

func (v *Validator) validateContainer(container *yaml.Node, containerNames map[string]bool) {
	if container.Kind != yaml.MappingNode {
		v.errorf(container.Line, "container must be object")
		return
	}

	fields := make(map[string]*yaml.Node)
	for i := 0; i < len(container.Content); i += 2 {
		if i+1 < len(container.Content) {
			key := container.Content[i]
			value := container.Content[i+1]
			fields[key.Value] = value
			fmt.Printf("DEBUG: Container field '%s' at line %d, value: %s\n", key.Value, key.Line, value.Value)
		}
	}

	requiredFields := []string{"name", "image", "resources"}
	for _, field := range requiredFields {
		if node, exists := fields[field]; !exists {
			v.errorf(container.Line, "%s is required", field)
		} else if field == "name" {
			v.validateContainerName(node, containerNames)
		} else if field == "image" {
			v.validateImage(node)
		}
	}

	if name, exists := fields["name"]; exists {
		v.validateContainerName(name, containerNames)
	}

	if image, exists := fields["image"]; exists {
		v.validateImage(image)
	}

	if ports, exists := fields["ports"]; exists {
		v.validatePorts(ports)
	}

	if readinessProbe, exists := fields["readinessProbe"]; exists {
		v.validateProbe(readinessProbe, "readinessProbe")
	}
	if livenessProbe, exists := fields["livenessProbe"]; exists {
		v.validateProbe(livenessProbe, "livenessProbe")
	}

	if resources, exists := fields["resources"]; exists {
		v.validateResources(resources)
	}
}

func (v *Validator) validateContainerName(name *yaml.Node, containerNames map[string]bool) {
	fmt.Printf("DEBUG: Validating container name '%s' at line %d\n", name.Value, name.Line)

	if name.Kind != yaml.ScalarNode {
		v.errorf(name.Line, "name must be string")
		return
	}

	// Проверка на пустую строку
	if strings.TrimSpace(name.Value) == "" {
		v.errorf(name.Line, "name is required")
		return
	}

	snakeCaseRegex := regexp.MustCompile(`^[a-z]+(_[a-z]+)*$`)
	if !snakeCaseRegex.MatchString(name.Value) {
		v.errorf(name.Line, "name has invalid format '%s'", name.Value)
		return
	}

	if containerNames[name.Value] {
		v.errorf(name.Line, "name '%s' is not unique", name.Value)
	} else {
		containerNames[name.Value] = true
	}
}

func (v *Validator) validateImage(image *yaml.Node) {
	if image.Kind != yaml.ScalarNode {
		v.errorf(image.Line, "image must be string")
		return
	}

	imageRegex := regexp.MustCompile(`^registry\.bigbrother\.io/[^:]+:.+$`)
	if !imageRegex.MatchString(image.Value) {
		v.errorf(image.Line, "image has invalid format '%s'", image.Value)
	}
}

func (v *Validator) validatePorts(ports *yaml.Node) {
	fmt.Printf("DEBUG: Validating ports at line %d\n", ports.Line)

	if ports.Kind != yaml.SequenceNode {
		v.errorf(ports.Line, "ports must be array")
		return
	}

	for i, port := range ports.Content {
		fmt.Printf("DEBUG: Validating port %d at line %d\n", i, port.Line)
		v.validateContainerPort(port)
	}
}

func (v *Validator) validateContainerPort(port *yaml.Node) {
	if port.Kind != yaml.MappingNode {
		v.errorf(port.Line, "port must be object")
		return
	}

	fields := make(map[string]*yaml.Node)
	for i := 0; i < len(port.Content); i += 2 {
		if i+1 < len(port.Content) {
			key := port.Content[i]
			value := port.Content[i+1]
			fields[key.Value] = value
			fmt.Printf("DEBUG: Port field '%s' at line %d, value: %s\n", key.Value, key.Line, value.Value)
		}
	}

	if containerPort, exists := fields["containerPort"]; !exists {
		v.errorf(port.Line, "containerPort is required")
	} else {
		fmt.Printf("DEBUG: Validating containerPort '%s' at line %d\n", containerPort.Value, containerPort.Line)
		v.validatePortNumber(containerPort, "containerPort")
	}

	if protocol, exists := fields["protocol"]; exists {
		v.validateString(protocol, "protocol", []string{"TCP", "UDP"})
	}
}

func (v *Validator) validateProbe(probe *yaml.Node, probeType string) {
	fmt.Printf("DEBUG: Validating %s at line %d\n", probeType, probe.Line)

	if probe.Kind != yaml.MappingNode {
		v.errorf(probe.Line, "%s must be object", probeType)
		return
	}

	fields := make(map[string]*yaml.Node)
	for i := 0; i < len(probe.Content); i += 2 {
		if i+1 < len(probe.Content) {
			key := probe.Content[i]
			value := probe.Content[i+1]
			fields[key.Value] = value
			fmt.Printf("DEBUG: %s field '%s' at line %d\n", probeType, key.Value, key.Line)
		}
	}

	if httpGet, exists := fields["httpGet"]; !exists {
		v.errorf(probe.Line, "httpGet is required")
	} else {
		v.validateHTTPGetAction(httpGet, probeType)
	}
}

func (v *Validator) validateHTTPGetAction(httpGet *yaml.Node, probeType string) {
	if httpGet.Kind != yaml.MappingNode {
		v.errorf(httpGet.Line, "httpGet must be object")
		return
	}

	fields := make(map[string]*yaml.Node)
	for i := 0; i < len(httpGet.Content); i += 2 {
		if i+1 < len(httpGet.Content) {
			key := httpGet.Content[i]
			value := httpGet.Content[i+1]
			fields[key.Value] = value
			fmt.Printf("DEBUG: httpGet field '%s' at line %d, value: %s\n", key.Value, key.Line, value.Value)
		}
	}

	if path, exists := fields["path"]; !exists {
		v.errorf(httpGet.Line, "path is required")
	} else {
		v.validateAbsolutePath(path, "path")
	}

	if port, exists := fields["port"]; !exists {
		v.errorf(httpGet.Line, "port is required")
	} else {
		fmt.Printf("DEBUG: Validating probe port '%s' at line %d\n", port.Value, port.Line)
		v.validatePortNumber(port, "port")
	}
}

func (v *Validator) validateResources(resources *yaml.Node) {
	if resources.Kind != yaml.MappingNode {
		v.errorf(resources.Line, "resources must be object")
		return
	}

	fields := make(map[string]*yaml.Node)
	for i := 0; i < len(resources.Content); i += 2 {
		if i+1 < len(resources.Content) {
			key := resources.Content[i]
			value := resources.Content[i+1]
			fields[key.Value] = value
		}
	}

	if requests, exists := fields["requests"]; exists {
		v.validateResourceRequirements(requests, "requests")
	}

	if limits, exists := fields["limits"]; exists {
		v.validateResourceRequirements(limits, "limits")
	}
}

func (v *Validator) validateResourceRequirements(resources *yaml.Node, fieldPath string) {
	if resources.Kind != yaml.MappingNode {
		v.errorf(resources.Line, "%s must be object", fieldPath)
		return
	}

	for i := 0; i < len(resources.Content); i += 2 {
		if i+1 < len(resources.Content) {
			key := resources.Content[i]
			value := resources.Content[i+1]

			switch key.Value {
			case "cpu":
				v.validateCPU(value, "cpu")
			case "memory":
				v.validateMemory(value, "memory")
			default:
				v.errorf(key.Line, "%s has unsupported resource '%s'", fieldPath, key.Value)
			}
		}
	}
}

func (v *Validator) validateCPU(cpu *yaml.Node, fieldPath string) {
	if cpu.Kind != yaml.ScalarNode {
		v.errorf(cpu.Line, "%s must be integer", fieldPath)
		return
	}

	// Убираем кавычки если они есть
	cleanedValue := strings.Trim(cpu.Value, `"`)
	if _, err := strconv.Atoi(cleanedValue); err != nil {
		v.errorf(cpu.Line, "%s must be integer", fieldPath)
	}
}

func (v *Validator) validateMemory(memory *yaml.Node, fieldPath string) {
	if memory.Kind != yaml.ScalarNode {
		v.errorf(memory.Line, "%s must be string", fieldPath)
		return
	}

	// Убираем кавычки если они есть
	cleanedValue := strings.Trim(memory.Value, `"`)
	memoryRegex := regexp.MustCompile(`^[0-9]+(Gi|Mi|Ki)$`)
	if !memoryRegex.MatchString(cleanedValue) {
		v.errorf(memory.Line, "%s has invalid format '%s'", fieldPath, cleanedValue)
	}
}

func (v *Validator) validateRequiredString(node *yaml.Node, fieldPath string) {
	fmt.Printf("DEBUG: Validating required string '%s' at line %d\n", node.Value, node.Line)

	if node.Kind != yaml.ScalarNode {
		v.errorf(node.Line, "%s must be string", fieldPath)
		return
	}

	// Проверка на пустую строку
	if strings.TrimSpace(node.Value) == "" {
		v.errorf(node.Line, "%s is required", fieldPath)
	}
}

func (v *Validator) validateString(node *yaml.Node, fieldPath string, allowedValues []string) {
	if node.Kind != yaml.ScalarNode {
		v.errorf(node.Line, "%s must be string", fieldPath)
		return
	}

	if allowedValues != nil {
		found := false
		for _, allowed := range allowedValues {
			if node.Value == allowed {
				found = true
				break
			}
		}
		if !found {
			v.errorf(node.Line, "%s has unsupported value '%s'", fieldPath, node.Value)
		}
	}
}

func (v *Validator) validatePortNumber(port *yaml.Node, fieldPath string) {
	fmt.Printf("DEBUG: Validating port number '%s' at line %d for field %s\n", port.Value, port.Line, fieldPath)

	if port.Kind != yaml.ScalarNode {
		v.errorf(port.Line, "%s must be integer", fieldPath)
		return
	}

	portNum, err := strconv.Atoi(port.Value)
	if err != nil {
		v.errorf(port.Line, "%s must be integer", fieldPath)
		return
	}

	if portNum <= 0 || portNum >= 65536 {
		v.errorf(port.Line, "%s value out of range", fieldPath)
	}
}

func (v *Validator) validateAbsolutePath(path *yaml.Node, fieldPath string) {
	if path.Kind != yaml.ScalarNode {
		v.errorf(path.Line, "%s must be string", fieldPath)
		return
	}

	if !strings.HasPrefix(path.Value, "/") {
		v.errorf(path.Line, "%s must be absolute path", fieldPath)
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <yaml-file>\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("%s: cannot read file: %v\n", filename, err)
		os.Exit(1)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		fmt.Printf("%s: cannot unmarshal YAML: %v\n", filename, err)
		os.Exit(1)
	}

	validator := &Validator{filename: filename}

	// Проверяем структуру документа
	fmt.Printf("DEBUG: Document has %d content nodes\n", len(root.Content))
	for i, doc := range root.Content {
		fmt.Printf("DEBUG: Document node %d: Kind=%d, Line=%d\n", i, doc.Kind, doc.Line)
		if doc.Kind == yaml.DocumentNode {
			fmt.Printf("DEBUG: DocumentNode has %d content nodes\n", len(doc.Content))
			if len(doc.Content) > 0 {
				validator.validateTopLevel(doc.Content[0])
			}
		} else {
			// Если это не DocumentNode, валидируем напрямую
			validator.validateTopLevel(doc)
		}
	}

	fmt.Printf("DEBUG: Total errors found: %d\n", len(validator.errors))

	if len(validator.errors) > 0 {
		for _, err := range validator.errors {
			fmt.Println(err)
		}
		os.Exit(1)
	}

	os.Exit(0)
}
