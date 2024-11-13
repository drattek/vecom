package com.vegusa.middleware.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "SystemParameter")
public class SystemParameter {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long recId;

    @Column(name = "Name", nullable = false, length = 50)
    private String parameterName;

    @Lob
    @Column(name = "DataType")
    private String dataType;

    @Column(name = "StrValue", length = 250)
    private String strValue;

    @Column(name = "IntValue")
    private Integer intValue;

    @Column(name = "Description", length = 250)
    private String description;

    @Column(name = "IntegrationCompany", nullable = false, length = 50)
    private String integrationCompany;

    public Long getRecId() {
        return recId;
    }

    public void setRecId(Long recId) {
        this.recId = recId;
    }

    public String getParameterName() {
        return parameterName;
    }

    public void setParameterName(String parameterName) {
        this.parameterName = parameterName;
    }

    public String getDataType() {
        return dataType;
    }

    public void setDataType(String dataType) {
        this.dataType = dataType;
    }

    public String getStrValue() {
        return strValue;
    }

    public void setStrValue(String strValue) {
        this.strValue = strValue;
    }

    public Integer getIntValue() {
        return intValue;
    }

    public void setIntValue(Integer intValue) {
        this.intValue = intValue;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getIntegrationCompany() {
        return integrationCompany;
    }

    public void setIntegrationCompany(String integrationCompany) {
        this.integrationCompany = integrationCompany;
    }

}