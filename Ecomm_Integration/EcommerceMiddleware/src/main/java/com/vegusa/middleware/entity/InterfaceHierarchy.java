package com.vegusa.middleware.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "interfacehierarchy")
public class InterfaceHierarchy {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", columnDefinition = "int UNSIGNED not null")
    private Long id;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "InterfaceRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "InterfaceId", referencedColumnName = "InterfaceId", nullable = false)
    })
    private InterfaceDS interfaceField;

    @Column(name = "Priority", nullable = false)
    private Integer priority;

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public InterfaceDS getInterfaceField() {
        return interfaceField;
    }

    public void setInterfaceField(InterfaceDS interfaceField) {
        this.interfaceField = interfaceField;
    }

    public Integer getPriority() {
        return priority;
    }

    public void setPriority(Integer priority) {
        this.priority = priority;
    }

}